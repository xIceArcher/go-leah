package qnap

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/docker/go-units"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

type API interface {
	Login(username string, password string) error
	Logout() error
	UploadMany(dir string, fileName string, filePaths []string) error
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
	resp, err := a.client.R().
		SetQueryParams(map[string]string{
			"user": username,
			"pwd":  base64.StdEncoding.EncodeToString([]byte(password)),
		}).
		SetResult(&LoginResponse{}).
		Get(a.url + "/wfm2Login.cgi")
	if err != nil {
		return err
	}

	loginResp := resp.Result().(*LoginResponse)
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

func (a *QNAPAPI) UploadMany(dir string, fileName string, filePaths []string) error {
	if a.sessionID == "" {
		return ErrNotLoggedIn
	}

	fileInfos, totalBytes, err := getFileSizes(filePaths)
	if err != nil {
		return err
	}

	logger := a.logger.With(zap.String("fileName", fileName))
	logger.Infof("Uploading file of size %v...", units.HumanSize(float64(totalBytes)))

	resp, err := a.postUtilRequest().
		SetQueryParams(map[string]string{
			"func":            "start_chunked_upload",
			"upload_root_dir": dir,
		}).
		SetResult(&StartChunkedUploadResponse{}).
		Post(a.utilRequestPath())
	if err != nil {
		return err
	}

	startChunkedUploadResp := resp.Result().(*StartChunkedUploadResponse)
	if startChunkedUploadResp.Status != StatusStartChunkedUploadOK {
		return ErrFailed
	}

	var offset int64
	for i, file := range fileInfos {
		for {
			resp, err = a.postUtilRequest().
				SetQueryParams(map[string]string{
					"func":            "chunked_upload",
					"upload_id":       startChunkedUploadResp.UploadID,
					"offset":          strconv.FormatInt(offset, 10),
					"overwrite":       "0",
					"dest_path":       dir,
					"upload_root_dir": dir,
					"filesize":        strconv.FormatInt(totalBytes, 10),
					"upload_name":     fileName,
					"multipart":       "1",
				}).
				SetFormData(map[string]string{
					"fileName": path.Base(file.Path),
				}).
				SetFile("file", file.Path).
				SetResult(&ChunkedUploadResponse{}).
				Post(a.utilRequestPath())
			if err != nil {
				logger.With(zap.Error(err)).Warn("Error uploading fragment")
				continue
			}

			chunkedUploadResp := resp.Result().(*ChunkedUploadResponse)
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

		logger.Infof("Uploaded fragment %v/%v", i+1, len(fileInfos))
	}

	return nil
}

func (a *QNAPAPI) postUtilRequest() *resty.Request {
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
