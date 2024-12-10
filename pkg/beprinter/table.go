package beprinter

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// print tables with column alignment
type TablePrinter struct {
	headers   []string
	rows      [][]string
	colWidths []int
}

// create new table printer
func NewTablePrinter(headers []string) *TablePrinter {
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	return &TablePrinter{
		headers:   headers,
		colWidths: colWidths,
	}
}

// insert new row to table
func (tp *TablePrinter) AddRow(row []string) {
	if len(row) != len(tp.headers) {
		log.Fatal("row length does not match headers length")
	}
	for i, cell := range row {
		if len(cell) > tp.colWidths[i] {
			tp.colWidths[i] = len(cell)
		}
	}
	tp.rows = append(tp.rows, row)
}

// print table
func (tp *TablePrinter) Print() {
	format := ""
	for _, width := range tp.colWidths {
		format += fmt.Sprintf("%%-%ds  ", width)
	}
	format = format[:len(format)-2] + "\n"

	// print header
	fmt.Printf(format, convertToAny(tp.headers)...)

	// print line delimiter
	for _, width := range tp.colWidths {
		fmt.Print(strings.Repeat("-", width+2))
	}
	fmt.Println()

	// print table rows
	for _, row := range tp.rows {
		fmt.Printf(format, convertToAny(row)...)
	}
}

// convert []string to []any for use in fmt.Printf
func convertToAny(slice []string) []any {
	result := make([]any, len(slice))
	for i, v := range slice {
		result[i] = v
	}

	return result
}
