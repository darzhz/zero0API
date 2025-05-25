package utils

import (
	"github.com/pocketbase/pocketbase/core"
)

func ToPublicList(records []*core.Record) []map[string]any {
	public := make([]map[string]any, len(records))
	for i, r := range records {
		public[i] = r.PublicExport()
	}
	return public
}
