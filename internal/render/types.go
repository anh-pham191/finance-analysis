package render

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type Format string

const (
	FormatTable    Format = "table"
	FormatCSV      Format = "csv"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "md"
)

type rangeJSON struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func renderJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(value)
}

func jsonRange(period domain.Range) rangeJSON {
	return rangeJSON{
		From: period.From,
		To:   period.To,
	}
}

func unknownFormatError(format Format) error {
	return fmt.Errorf("unknown render format %q", format)
}
