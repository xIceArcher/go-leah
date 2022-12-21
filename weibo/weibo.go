package weibo

import (
	"github.com/go-resty/resty/v2"
)

type API struct {
	client *resty.Client
}

func NewAPI() *API {
	return &API{
		client: resty.New().SetHeader(
			"Referer", "https://weibo.com",
		),
	}
}

func (a *API) GetPost(id string) (*Post, error) {
	resp := &RawGetPostResp{}

	_, err := a.client.R().
		SetQueryParam("id", id).
		SetResult(resp).
		Get("https://weibo.com/ajax/statuses/show")
	if err != nil {
		return nil, err
	}

	ret := &Post{
		ID:     id,
		Photos: make([]*Photo, 0, len(resp.PicInfos)),
	}

	for _, picID := range resp.PicIDs {
		rawPhotoMap, ok := resp.PicInfos[picID]
		if !ok {
			continue
		}

		photo := &Photo{
			ID: picID,
		}

		for variantName, variant := range rawPhotoMap {
			photo.Variants = append(photo.Variants, &PhotoVariant{
				VariantName: variantName,
				URL:         variant.URL,
				Height:      variant.Height,
				Width:       variant.Width,
			})
		}

		ret.Photos = append(ret.Photos, photo)
	}

	return ret, nil
}

func (a *API) GetPhotoVariantSize(photoVariant *PhotoVariant) (int64, error) {
	resp, err := a.client.R().Head(photoVariant.URL)
	if err != nil {
		return 0, err
	}
	return resp.RawResponse.ContentLength, nil
}

func (a *API) DownloadPhotoVariant(photoVariant *PhotoVariant) ([]byte, error) {
	resp, err := a.client.R().Get(photoVariant.URL)
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}
