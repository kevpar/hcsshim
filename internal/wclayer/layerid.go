package wclayer

import (
	"github.com/Microsoft/go-winio/pkg/guid"
)

// LayerID returns the layer ID of a layer on disk.
func LayerID(path string) (guid.GUID, error) {
	return NameToGuid(path)
}
