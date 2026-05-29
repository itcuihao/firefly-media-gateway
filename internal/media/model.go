package media

import "time"

const (
	ProviderTG = "tg"
	ProviderR2 = "r2"
)

const (
	StatusActive  = "active"
	StatusDeleted = "deleted"
)

type Asset struct {
	ID                   string     `json:"mediaId"`
	Provider             string     `json:"provider"`
	ProviderFileID       string     `json:"-"`
	ProviderBucketOrChat *string    `json:"-"`
	PublicURL            string     `json:"publicUrl"`
	MIMEType             string     `json:"mimeType"`
	SizeBytes            int64      `json:"sizeBytes"`
	SHA256               *string    `json:"sha256,omitempty"`
	Project              string     `json:"project"`
	Usage                string     `json:"usage"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
	IsChunked            bool       `json:"isChunked,omitempty"`
}

type Chunk struct {
	AssetID      string `json:"assetId"`
	ChunkIndex   int    `json:"chunkIndex"`
	ChunkFileID  string `json:"chunkFileId"`
}

type CreateAssetInput struct {
	ID                   string
	Provider             string
	ProviderFileID       string
	ProviderBucketOrChat *string
	PublicURL            string
	MIMEType             string
	SizeBytes            int64
	SHA256               *string
	Project              string
	Usage                string
	IsChunked            bool
}
