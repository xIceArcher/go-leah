package qnap

type Status int

type StatusMixin struct {
	Status Status `json:"status"`
}

type LoginResponse struct {
	StatusMixin
	SID string `json:"sid"`
}

type CreateDirResponse struct {
	StatusMixin
}

type UploadResponse struct {
	StatusMixin
}

type StartChunkedUploadResponse struct {
	StatusMixin
	UploadID string `json:"upload_id"`
}

type ChunkedUploadResponse struct {
	StatusMixin
	Size string `json:"size"`
}

type StatResponse struct {
	Datas []StatData `json:"datas"`
}

type StatData struct {
	FileName string `json:"filename"`
	FileSize string `json:"filesize"`
	Exist    int    `json:"exist"`
}

func (d *StatData) IsExist() bool {
	return d.Exist != 0
}
