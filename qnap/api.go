package qnap

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/docker/go-units"
	"github.com/go-resty/resty/v2"
	"github.com/xIceArcher/go-leah/progress"
	"go.uber.org/zap"
)

type API interface {
	APIAuthenticator

	CreateDir(rootDir string, dirName string) error
	UploadMany(dir string, filePaths []string, progressBar ...progress.Bar) error
	UploadAndConcat(dir string, fileName string, filePaths []string, progressBar ...progress.Bar) (int64, error)
	GetFileSize(path string, fileName string) (int64, error)
	Exists(path string, fileName string) (bool, error)
}

type APIAuthenticator interface {
	Login(username string, password string) error
	Logout() error

	SessionID() string
}

type QNAPAPIAuthenticatorV4_1 struct {
	url       string
	sessionID string

	client *resty.Client
}

func NewV4_1Authenticator(baseUrl string) (APIAuthenticator, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "cgi-bin", "filemanager")

	client := resty.New().
		SetTimeout(0).
		SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})

	return &QNAPAPIAuthenticatorV4_1{
		url:    u.String(),
		client: client,
	}, nil
}

func (a *QNAPAPIAuthenticatorV4_1) Login(username string, password string) error {
	loginResp := &V4_1LoginResponse{}
	_, err := a.client.R().
		SetQueryParams(map[string]string{
			"user": username,
			"pwd":  base64.StdEncoding.EncodeToString([]byte(password)),
		}).
		SetResult(loginResp).
		Get(a.url + "/wfm2Login.cgi")
	if err != nil {
		return err
	}

	if loginResp.Status != StatusOK {
		return ErrFailed
	}

	a.sessionID = loginResp.SID
	return nil
}

func (a *QNAPAPIAuthenticatorV4_1) Logout() error {
	if a.sessionID == "" {
		return nil
	}

	_, err := a.client.R().Get(a.url + "/wfm2Logout.cgi")
	return err
}

func (a *QNAPAPIAuthenticatorV4_1) SessionID() string {
	return a.sessionID
}

type QNAPAPIAuthenticatorV5 struct {
	url       string
	sessionID string

	client *resty.Client
}

func NewV5Authenticator(baseUrl string) (APIAuthenticator, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "cgi-bin")

	client := resty.New().
		SetTimeout(0).
		SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})

	return &QNAPAPIAuthenticatorV5{
		url:    u.String(),
		client: client,
	}, nil
}

func (a *QNAPAPIAuthenticatorV5) Login(username string, password string) error {
	loginResp := &V5LoginResponse{}
	_, err := a.client.R().
		SetQueryParams(map[string]string{
			"user": username,
			"pwd":  base64.StdEncoding.EncodeToString([]byte(password)),
		}).
		SetResult(loginResp).
		Get(a.url + "/authLogin.cgi")
	if err != nil {
		return err
	}

	if !loginResp.AuthPassed {
		return ErrFailed
	}

	a.sessionID = loginResp.AuthSid
	return nil
}

func (a *QNAPAPIAuthenticatorV5) Logout() error {
	return nil
}

func (a *QNAPAPIAuthenticatorV5) SessionID() string {
	return a.sessionID
}

type QNAPAPI struct {
	APIAuthenticator

	url string

	client *resty.Client
	logger *zap.SugaredLogger
}

func New(baseUrl string, logger *zap.SugaredLogger) (API, error) {
	defaultAuthenticator, err := NewV5Authenticator(baseUrl)
	if err != nil {
		return nil, err
	}

	return NewWithAuthenticator(baseUrl, logger, defaultAuthenticator)
}

func NewWithAuthenticator(baseUrl string, logger *zap.SugaredLogger, autheticator APIAuthenticator) (API, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "cgi-bin", "filemanager")

	client := resty.New().
		SetTimeout(0).
		SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})

	return &QNAPAPI{
		APIAuthenticator: autheticator,
		url:              u.String(),
		client:           client,
		logger:           logger,
	}, nil
}

type FileInfo struct {
	Path     string
	NumBytes int64
}

func (a *QNAPAPI) CreateDir(rootDir string, dirName string) error {
	if a.SessionID() == "" {
		return ErrNotLoggedIn
	}

	createDirResp := &CreateDirResponse{}
	_, err := a.utilRequest().
		SetQueryParams(map[string]string{
			"func": "createdir",
		}).
		SetFormData(map[string]string{
			"dest_path":   rootDir,
			"dest_folder": dirName,
		}).
		SetResult(createDirResp).
		Post(a.utilRequestPath())
	if err != nil {
		return err
	}

	if createDirResp.Status != StatusOK {
		return ErrFailed
	}

	return nil
}

func (a *QNAPAPI) UploadMany(dirName string, filePaths []string, bar ...progress.Bar) error {
	var progressBar progress.Bar
	if len(bar) > 0 {
		progressBar = bar[0]
	}

	if a.SessionID() == "" {
		return ErrNotLoggedIn
	}

	fileInfos, totalBytes, err := getFileSizes(filePaths)
	if err != nil {
		return err
	}

	if progressBar != nil {
		progressBar.SetMax(totalBytes)
	}

	logger := a.logger.With(zap.String("dirName", dirName))
	logger.Infof("Uploading file of size %v...", units.HumanSize(float64(totalBytes)))

	for i, file := range fileInfos {
		uploadResp := &UploadResponse{}

		if _, err := a.utilRequest().
			SetQueryParams(map[string]string{
				"func":      "upload",
				"type":      "standard",
				"dest_path": dirName,
				"overwrite": "0",
				"progress":  path.Base(file.Path),
			}).
			SetFile("file", file.Path).
			SetResult(uploadResp).
			Post(a.utilRequestPath()); err != nil {
			return err
		}
		if uploadResp.Status != StatusOK {
			logger.Warnf("Upload returned error %v", uploadResp.Status)
			continue
		}

		if progressBar != nil {
			progressBar.Add(file.NumBytes)
		}
		logger.Infof("Uploaded file %v/%v", i+1, len(fileInfos))
	}

	return nil
}

func (a *QNAPAPI) UploadAndConcat(uploadDir string, fileName string, filePaths []string, bar ...progress.Bar) (int64, error) {
	var progressBar progress.Bar
	if len(bar) > 0 {
		progressBar = bar[0]
	}

	if a.SessionID() == "" {
		return 0, ErrNotLoggedIn
	}

	fileInfos, totalBytes, err := getFileSizes(filePaths)
	if err != nil {
		return 0, err
	}

	if progressBar != nil {
		progressBar.SetMax(totalBytes)
	}

	logger := a.logger.With(zap.String("fileName", fileName))
	logger.Infof("Uploading file of size %v...", units.HumanSize(float64(totalBytes)))

	startChunkedUploadResp := &StartChunkedUploadResponse{}
	_, err = a.utilRequest().
		SetQueryParams(map[string]string{
			"func":            "start_chunked_upload",
			"upload_root_dir": uploadDir,
		}).
		SetResult(startChunkedUploadResp).
		Post(a.utilRequestPath())
	if err != nil {
		return 0, err
	}

	if startChunkedUploadResp.Status != StatusStartChunkedUploadOK {
		return 0, ErrFailed
	}

	var offset int64
	for i, file := range fileInfos {
		start := time.Now()

		for {
			chunkedUploadResp := &ChunkedUploadResponse{}
			_, err := a.utilRequest().
				SetQueryParams(map[string]string{
					"func":            "chunked_upload",
					"upload_id":       startChunkedUploadResp.UploadID,
					"offset":          strconv.FormatInt(offset, 10),
					"overwrite":       "0",
					"dest_path":       uploadDir,
					"upload_root_dir": uploadDir,
					"filesize":        strconv.FormatInt(totalBytes, 10),
					"upload_name":     fileName,
					"multipart":       "1",
				}).
				SetFormData(map[string]string{
					"fileName": path.Base(file.Path),
				}).
				SetFile("file", file.Path).
				SetResult(chunkedUploadResp).
				Post(a.utilRequestPath())
			if err == io.EOF {
				logger.Warn("Received EOF")
				break
			} else if err != nil {
				logger.With(zap.Error(err)).Warn("Error uploading fragment")
				continue
			}

			if chunkedUploadResp.Status != StatusOK {
				logger.Warnf("Chunked upload returned error %v", chunkedUploadResp.Status)
				continue
			}

			actualSize, err := strconv.ParseInt(chunkedUploadResp.Size, 10, 64)
			if err != nil {
				logger.With(zap.Error(err)).Warn("Failed to parse size %s", actualSize)
				continue
			}

			if offset+file.NumBytes != actualSize {
				logger.Warnf("Expected %v bytes but got %v bytes", offset+file.NumBytes, actualSize)
				continue
			}

			offset += file.NumBytes
			break
		}

		if progressBar != nil {
			progressBar.Add(file.NumBytes)
		}
		logger.With("took", time.Since(start)).Infof("Uploaded fragment %v/%v", i+1, len(fileInfos))
	}

	actualFileSize, err := a.GetFileSize(uploadDir, fileName)
	if err != nil {
		return 0, err
	}

	if actualFileSize != totalBytes {
		return 0, ErrFailed
	}

	return actualFileSize, nil
}

func (a *QNAPAPI) GetFileSize(path string, fileName string) (int64, error) {
	if a.SessionID() == "" {
		return 0, ErrNotLoggedIn
	}

	statResp := &StatResponse{}
	_, err := a.utilRequest().
		SetQueryParams(map[string]string{
			"func":       "stat",
			"path":       path,
			"file_total": "1",
			"file_name":  fileName,
		}).
		SetResult(statResp).
		Get(a.utilRequestPath())
	if err != nil {
		return 0, err
	}

	if len(statResp.Datas) == 0 || !statResp.Datas[0].IsExist() || statResp.Datas[0].FileName != fileName {
		return 0, ErrNotFound
	}

	fileSize, err := strconv.ParseInt(statResp.Datas[0].FileSize, 10, 64)
	if err != nil {
		return 0, err
	}

	return fileSize, nil
}

func (a *QNAPAPI) Exists(path string, fileName string) (bool, error) {
	_, err := a.GetFileSize(path, fileName)
	if err == nil {
		return true, nil
	} else if errors.Is(err, ErrNotFound) {
		return false, nil
	}

	return false, err
}

func (a *QNAPAPI) utilRequest() *resty.Request {
	return a.client.R().SetQueryParam("sid", a.SessionID())
}

func (a *QNAPAPI) utilRequestPath() string {
	return a.url + "/utilRequest.cgi"
}

func getFileSizes(fileNames []string) (fileInfos []*FileInfo, totalBytes int64, err error) {
	for _, fileName := range fileNames {
		fi, err := os.Stat(fileName)
		if err != nil {
			return nil, 0, err
		}

		fileInfos = append(fileInfos, &FileInfo{
			Path:     fileName,
			NumBytes: fi.Size(),
		})

		totalBytes += fi.Size()
	}

	return
}
