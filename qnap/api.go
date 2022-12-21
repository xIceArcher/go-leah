package qnap

import (
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/docker/go-units"
	"github.com/go-resty/resty/v2"
	"github.com/xIceArcher/go-leah/progress"
	"go.uber.org/zap"
)

type API interface {
	Login(username string, password string) error
	Logout() error
	CreateDir(rootDir string, dirName string) error
	UploadMany(dir string, filePaths []string, progressBar ...progress.Bar) error
	UploadAndConcat(dir string, fileName string, filePaths []string, progressBar ...progress.Bar) error
	GetFileSize(path string, fileName string) (int64, error)
}

type QNAPAPI struct {
	url       string
	sessionID string

	client *resty.Client
	logger *zap.SugaredLogger
}

func New(baseUrl string, logger *zap.SugaredLogger) (API, error) {
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
		client: client,

		url:    u.String(),
		logger: logger,
	}, nil
}

func (a *QNAPAPI) Login(username string, password string) error {
	loginResp := &LoginResponse{}
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

func (a *QNAPAPI) Logout() error {
	if a.sessionID == "" {
		return nil
	}

	_, err := a.client.R().Get(a.url + "/wfm2Logout.cgi")
	return err
}

type FileInfo struct {
	Path     string
	NumBytes int64
}

func (a *QNAPAPI) CreateDir(rootDir string, dirName string) error {
	if a.sessionID == "" {
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

	if a.sessionID == "" {
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

func (a *QNAPAPI) UploadAndConcat(uploadDir string, fileName string, filePaths []string, bar ...progress.Bar) error {
	var progressBar progress.Bar
	if len(bar) > 0 {
		progressBar = bar[0]
	}

	if a.sessionID == "" {
		return ErrNotLoggedIn
	}

	fileInfos, totalBytes, err := getFileSizes(filePaths)
	if err != nil {
		return err
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
		return err
	}

	if startChunkedUploadResp.Status != StatusStartChunkedUploadOK {
		return ErrFailed
	}

	var offset int64
	for i, file := range fileInfos {
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
				continue
			}

			offset += file.NumBytes
			break
		}

		if progressBar != nil {
			progressBar.Add(file.NumBytes)
		}
		logger.Infof("Uploaded fragment %v/%v", i+1, len(fileInfos))
	}

	actualFileSize, err := a.GetFileSize(uploadDir, fileName)
	if err != nil {
		return err
	}

	if actualFileSize != totalBytes {
		return ErrFailed
	}

	return nil
}

func (a *QNAPAPI) GetFileSize(path string, fileName string) (int64, error) {
	if a.sessionID == "" {
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

	if len(statResp.Datas) == 0 || statResp.Datas[0].FileName != fileName {
		return 0, ErrNotFound
	}

	fileSize, err := strconv.ParseInt(statResp.Datas[0].FileSize, 10, 64)
	if err != nil {
		return 0, err
	}

	return fileSize, nil
}

func (a *QNAPAPI) utilRequest() *resty.Request {
	return a.client.R().SetQueryParam("sid", a.sessionID)
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
