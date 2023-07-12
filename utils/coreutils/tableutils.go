package coreutils

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/term"
)

// Controls the max col width when printing to a non-terminal. See the PrintTable description for more info.
var DefaultMaxColWidth = 25

// PrintTable prints a slice of rows in a table.
// The parameter rows MUST be a slice, otherwise the method panics.
// How to use this method (with an example):
// The fields of the struct must have one of the tags: 'col-name' or 'embed-table' in order to be printed.
// Fields without any of these tags will be skipped.
// The tag 'col-name' can be set on string fields only. The column name is the 'col-name' tag value.
// On terminal, the maximum column width is calculated (terminal width equally divided between columns),
// while on non-terminal the value of the DefaultMaxColWidth variable is used.
// If the cell content exceeds the defined max column width, the content will be broken into two (or more) lines.
// In case the struct you want to print contains a field that is a slice of other structs,
// you can print it in the table too with the 'embed-table' tag which can be set on slices of structs only.
// Fields with the 'extended' tag will be printed iff the 'printExtended' bool input is true.
// You can merge cells horizontally with the 'auto-merge' tag, it will merge cells with the same value.
//
// Example:
// These are the structs Customer and Product:
//
//	type Customer struct {
//	    name     string    `col-name:"Name"`
//	    age      string    `col-name:"Age"`
//	    products []Product `embed-table:"true"`
//	}
//
//	type Product struct {
//	    title string `col-name:"Product Title"`
//	    CatNumber string `col-name:"Product\nCatalog #"`
//	    Color string `col-name:"Color" extended:"true"`
//	}
//
// We'll use it, and run these commands (var DefaultMaxColWidth = 25):
//
//	customersSlice := []Customer{
//	    {name: "Gai", age: "350", products: []Product{{title: "SpiderFrog Shirt - Medium", CatNumber: "123456", Color: "Green"}, {title: "Floral Bottle", CatNumber: "147585", Color: "Blue"}}},
//	    {name: "Noah", age: "21", products: []Product{{title: "Pouch", CatNumber: "456789", Color: "Red"}, {title: "Ching Ching", CatNumber: "963852", Color: "Gold"}}},
//	}
//
// err := coreutils.PrintTable(customersSlice, "Customers", "No customers were found", false)
//
// That's the table printed:
//
// Customers
// ┌──────┬─────┬─────────────────────────┬───────────┐
// │ NAME │ AGE │ PRODUCT TITLE           │ PRODUCT   │
// │      │     │                         │ CATALOG # │
// ├──────┼─────┼─────────────────────────┼───────────┤
// │ Gai  │ 350 │ SpiderFrog Shirt - Medi │ 123456    │
// │      │     │ um                      │           │
// │      │     │ Floral Bottle           │ 147585    │
// ├──────┼─────┼─────────────────────────┼───────────┤
// │ Noah │ 21  │ Pouch                   │ 456789    │
// │      │     │ Ching Ching             │ 963852    │
// └──────┴─────┴─────────────────────────┴───────────┘
//
// If printExtended=true:
//
// err := coreutils.PrintTable(customersSlice, "Customers", "No customers were found", true)
//
// Customers
// ┌──────┬─────┬─────────────────────────┬───────────┬───────────┐
// │ NAME │ AGE │ PRODUCT TITLE           │ PRODUCT   │ Color     │
// │      │     │                         │ CATALOG # │           │
// ├──────┼─────┼─────────────────────────┼───────────┼───────────┤
// │ Gai  │ 350 │ SpiderFrog Shirt - Medi │ 123456    │ Green     │
// │      │     │ um                      │           │           │
// │      │     │ Floral Bottle           │ 147585    │ Blue      │
// ├──────┼─────┼─────────────────────────┼───────────┼───────────┤
// │ Noah │ 21  │ Pouch                   │ 456789    │ Red       │
// │      │     │ Ching Ching             │ 963852    │ Gold      │
// └──────┴─────┴─────────────────────────┴───────────┴───────────┘
//
// If customersSlice was empty, emptyTableMessage would have been printed instead:
//
// Customers
// ┌─────────────────────────┐
// │ No customers were found │
// └─────────────────────────┘
//
// Example(auto-merge):
// These are the structs Customer:
//
//	type Customer struct {
//	    name     string    `col-name:"Name" auto-merge:"true"`
//	    age       string   `col-name:"Age" auto-merge:"true"`
//	    title     string   `col-name:"Product Title" auto-merge:"true"`
//	    CatNumber string   `col-name:"Product\nCatalog #" auto-merge:"true"`
//	    Color     string   `col-name:"Color" extended:"true" auto-merge:"true"`
//	}
//
//  customersSlice := []Customer{
//	    {name: "Gai", age: "350", title: "SpiderFrog Shirt - Medium", CatNumber: "123456", Color: "Green"},
//      {name: "Gai", age: "350", title: "Floral Bottle", CatNumber: "147585", Color: "Blue"},
//	    {name: "Noah", age: "21", title: "Pouch", CatNumber: "456789", Color: "Red"},
// }
//
// Customers
// ┌──────┬─────┬───────────────────────────┬───────────┐
// │ NAME │ AGE │ PRODUCT TITLE             │ PRODUCT   │
// │      │     │                           │ CATALOG # │
// ├──────┼─────┼───────────────────────────┼───────────┤
// │ Gai  │ 350 │ SpiderFrog Shirt - Medium │ 123456    │
// │      │     ├───────────────────────────┼───────────┤
// │      │     │ Floral Bottle             │ 147585    │
// ├──────┼─────┼───────────────────────────┼───────────┤
// │ Noah │ 21  │ Pouch                     │ 456789    │
// └──────┴─────┴───────────────────────────┴───────────┘

func PrintTable(rows interface{}, title string, emptyTableMessage string, printExtended bool) (err error) {
	tableWriter, err := PrepareTable(rows, emptyTableMessage, printExtended)
	if err != nil || tableWriter == nil {
		return
	}

	if title != "" {
		log.Output(title)
	}

	if log.IsStdOutTerminal() || os.Getenv("GITLAB_CI") == "" {
		tableWriter.SetStyle(table.StyleLight)
	}
	tableWriter.Style().Options.SeparateRows = true
	stdoutWriter := bufio.NewWriter(os.Stdout)
	defer func() {
		e := stdoutWriter.Flush()
		if err == nil {
			err = e
		}
	}()
	tableWriter.SetOutputMirror(stdoutWriter)
	tableWriter.Render()
	return
}

// Creates table following the logic described in PrintTable.
// Returns:
// Table Writer (table.Writer) - Can be used to adjust style, output mirror, render type, etc.
// Error if occurred.
func PrepareTable(rows interface{}, emptyTableMessage string, printExtended bool) (table.Writer, error) {
	tableWriter := table.NewWriter()

	rowsSliceValue := reflect.ValueOf(rows)
	if rowsSliceValue.Len() == 0 && emptyTableMessage != "" {
		PrintMessage(emptyTableMessage)
		return nil, nil
	}

	rowType := reflect.TypeOf(rows).Elem()
	fieldsCount := rowType.NumField()
	var columnsNames []interface{}
	var fieldsProperties []fieldProperties
	var columnConfigs []table.ColumnConfig
	for i := 0; i < fieldsCount; i++ {
		field := rowType.Field(i)
		columnName, columnNameExist := field.Tag.Lookup("col-name")
		embedTable, embedTableExist := field.Tag.Lookup("embed-table")
		extended, extendedExist := field.Tag.Lookup("extended")
		_, autoMerge := field.Tag.Lookup("auto-merge")
		_, omitEmptyColumn := field.Tag.Lookup("omitempty")
		if !printExtended && extendedExist && extended == "true" {
			continue
		}
		if !columnNameExist && !embedTableExist {
			continue
		}
		if omitEmptyColumn && isColumnEmpty(rowsSliceValue, i) {
			continue
		}
		if embedTable == "true" {
			var subfieldsProperties []subfieldProperties
			var err error
			columnsNames, columnConfigs, subfieldsProperties = appendEmbeddedTableFields(columnsNames, columnConfigs, field, printExtended)
			if err != nil {
				return nil, err
			}
			fieldsProperties = append(fieldsProperties, fieldProperties{index: i, subfields: subfieldsProperties})
		} else {
			columnsNames = append(columnsNames, columnName)
			fieldsProperties = append(fieldsProperties, fieldProperties{index: i})
			columnConfigs = append(columnConfigs, table.ColumnConfig{Name: columnName, AutoMerge: autoMerge})
		}
	}
	tableWriter.AppendHeader(columnsNames)
	err := setColMaxWidth(columnConfigs, fieldsProperties)
	if err != nil {
		return nil, err
	}
	tableWriter.SetColumnConfigs(columnConfigs)

	for i := 0; i < rowsSliceValue.Len(); i++ {
		var rowValues []interface{}
		currRowValue := rowsSliceValue.Index(i)
		for _, fieldProps := range fieldsProperties {
			currField := currRowValue.Field(fieldProps.index)
			if len(fieldProps.subfields) > 0 {
				rowValues = appendEmbeddedTableStrings(rowValues, currField, fieldProps.subfields)
			} else {
				rowValues = append(rowValues, currField.String())
			}
		}
		tableWriter.AppendRow(rowValues)
	}

	return tableWriter, nil
}

func isColumnEmpty(rows reflect.Value, fieldIndex int) bool {
	for i := 0; i < rows.Len(); i++ {
		currRowValue := rows.Index(i)
		currField := currRowValue.Field(fieldIndex)
		if currField.String() != "" {
			return false
		}
	}
	return true
}

type fieldProperties struct {
	index     int                  // The location of the field inside the row struct
	subfields []subfieldProperties // If this field is an embedded table, this will contain the fields in it
}

type subfieldProperties struct {
	index    int
	maxWidth int
}

func setColMaxWidth(columnConfigs []table.ColumnConfig, fieldsProperties []fieldProperties) error {
	colMaxWidth := DefaultMaxColWidth

	// If terminal, calculate the max width.
	if log.IsStdOutTerminal() {
		colNum := len(columnConfigs)
		termWidth, err := getTerminalAllowedWidth(colNum)
		if err != nil {
			return err
		}
		colMaxWidth = int(math.Floor(float64(termWidth) / float64(colNum)))
	}

	// Set the max width on every column and cell.
	for i := range columnConfigs {
		columnConfigs[i].WidthMax = colMaxWidth
	}
	for i := range fieldsProperties {
		subfields := fieldsProperties[i].subfields
		for j := range subfields {
			subfields[j].maxWidth = colMaxWidth
		}
	}
	return nil
}

func getTerminalAllowedWidth(colNum int) (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, err
	}
	// Subtract the table's grid chars (3 chars between every two columns and 1 char at both edges of the table).
	subtraction := (colNum-1)*3 + 2
	return width - subtraction, nil
}

func appendEmbeddedTableFields(columnsNames []interface{}, columnConfigs []table.ColumnConfig, field reflect.StructField, printExtended bool) ([]interface{}, []table.ColumnConfig, []subfieldProperties) {
	rowType := field.Type.Elem()
	fieldsCount := rowType.NumField()
	var subfieldsProperties []subfieldProperties
	for i := 0; i < fieldsCount; i++ {
		innerField := rowType.Field(i)
		columnName, columnNameExist := innerField.Tag.Lookup("col-name")
		extended, extendedExist := innerField.Tag.Lookup("extended")
		if !printExtended && extendedExist && extended == "true" {
			continue
		}
		if !columnNameExist {
			continue
		}
		columnsNames = append(columnsNames, columnName)
		columnConfigs = append(columnConfigs, table.ColumnConfig{Name: columnName})
		subfieldsProperties = append(subfieldsProperties, subfieldProperties{index: i})
	}
	return columnsNames, columnConfigs, subfieldsProperties
}

func appendEmbeddedTableStrings(rowValues []interface{}, fieldValue reflect.Value, subfieldsProperties []subfieldProperties) []interface{} {
	sliceLen := fieldValue.Len()
	numberOfColumns := len(subfieldsProperties)
	tableStrings := make([]string, numberOfColumns)

	for rowIndex := 0; rowIndex < sliceLen; rowIndex++ {
		currRowCells := make([]embeddedTableCell, numberOfColumns)
		maxNumberOfLines := 0

		// Check if all elements in the row are empty.
		shouldSkip := true
		for _, subfieldProps := range subfieldsProperties {
			currCellContent := fieldValue.Index(rowIndex).Field(subfieldProps.index).String()
			if currCellContent != "" {
				shouldSkip = false
				break
			}
		}
		// Skip row if no non-empty cell was found.
		if shouldSkip {
			continue
		}

		// Find the highest number of lines in the row
		for subfieldIndex, subfieldProps := range subfieldsProperties {
			currCellContent := fieldValue.Index(rowIndex).Field(subfieldProps.index).String()
			currRowCells[subfieldIndex] = embeddedTableCell{content: currCellContent, numberOfLines: countLinesInCell(currCellContent, subfieldProps.maxWidth)}
			if currRowCells[subfieldIndex].numberOfLines > maxNumberOfLines {
				maxNumberOfLines = currRowCells[subfieldIndex].numberOfLines
			}
		}

		// Add newlines to cells with less lines than maxNumberOfLines
		for colIndex, currCell := range currRowCells {
			cellContent := currCell.content
			for i := 0; i < maxNumberOfLines-currCell.numberOfLines; i++ {
				cellContent = fmt.Sprintf("%s\n", cellContent)
			}
			tableStrings[colIndex] = fmt.Sprintf("%s%s\n", tableStrings[colIndex], cellContent)
		}
	}
	for _, tableString := range tableStrings {
		trimmedString := strings.TrimSuffix(tableString, "\n")
		rowValues = append(rowValues, trimmedString)
	}
	return rowValues
}

func countLinesInCell(content string, maxWidth int) int {
	if maxWidth == 0 {
		return strings.Count(content, "\n") + 1
	}
	lines := strings.Split(content, "\n")
	numberOfLines := 0
	for _, line := range lines {
		numberOfLines += len(line) / maxWidth
		if len(line)%maxWidth > 0 {
			numberOfLines++
		}
	}
	return numberOfLines
}

type embeddedTableCell struct {
	content       string
	numberOfLines int
}

// PrintMessage prints message in a frame (which is actually a table with a single table).
// For example:
// ┌─────────────────────────────────────────┐
// │ An example of a message in a nice frame │
// └─────────────────────────────────────────┘
func PrintMessage(message string) {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	if log.IsStdOutTerminal() {
		tableWriter.SetStyle(table.StyleLight)
	}
	// Remove emojis from non-supported terminals
	message = RemoveEmojisIfNonSupportedTerminal(message)
	tableWriter.AppendRow(table.Row{message})
	tableWriter.Render()
}
