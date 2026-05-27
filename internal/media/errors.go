package media

import "errors"

var (
	ErrNotFound         = errors.New("media asset not found")
	ErrInvalidFileType  = errors.New("invalid file type")
	ErrFileTooLarge     = errors.New("file too large")
	ErrProviderDisabled = errors.New("provider disabled")
)
