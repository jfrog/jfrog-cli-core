package coreutils

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// PrintTable prints a slice of rows in a table.
// The parameter rows MUST be a slice, otherwise the method panics.
// How to use this method (with an example):
// The fields of the struct must have one of the tags: 'col-name' or 'embed-table' in order to be printed.
// Fields without any of these tags will be skipped.
// The tag 'col-name' can be set on string fields only. The column name is the 'col-name' tag value.
// If the cell content exceeds the defined col-max-width, the content will be broken into two (or more) lines.
// If you would like to limit the width of the column, you can use the 'col-max-width' tag.
// In case the struct you want to print contains a field that is a slice of other structs,
// you can print it in the table too with the 'embed-table' tag which can be set on slices of structs only.
//
// Example:
// These are the structs Customer and Product:
//
// type Customer struct {
//     name     string    `col-name:"Name"`
//     age      string    `col-name:"Age"`
//     products []Product `embed-table:"true"`
// }
//
// type Product struct {
//     title string `col-name:"Product Title" col-max-width:"15"`
//     CatNumber string `col-name:"Product\nCatalog #"`
// }
//
// We'll use it, and run these commands:
//
// customersSlice := []Customer{
//     {name: "Gai", age: "350", products: []Product{{title: "SpiderFrog Shirt - Medium", CatNumber: "123456"}, {title: "Floral Bottle", CatNumber: "147585"}}},
//     {name: "Noah", age: "21", products: []Product{{title: "Pouch", CatNumber: "456789"}, {title: "Ching Ching", CatNumber: "963852"}}},
// }
// err := coreutils.PrintTable(customersSlice, "Customers", "No customers were found")
//
// That's the table printed:
//
// Customers
// ┌──────┬─────┬─────────────────┬───────────┐
// │ NAME │ AGE │ PRODUCT TITLE   │ PRODUCT   │
// │      │     │                 │ CATALOG # │
// ├──────┼─────┼─────────────────┼───────────┤
// │ Gai  │ 350 │ SpiderFrog Shir │ 123456    │
// │      │     │ t - Medium      │           │
// │      │     │ Floral Bottle   │ 147585    │
// ├──────┼─────┼─────────────────┼───────────┤
// │ Noah │ 21  │ Pouch           │ 456789    │
// │      │     │ Ching Ching     │ 963852    │
// └──────┴─────┴─────────────────┴───────────┘
//
// If customersSlice was empty, emptyTableMessage would have been printed instead:
//
// Customers
// ┌─────────────────────────┐
// │ No customers were found │
// └─────────────────────────┘
func PrintTable(rows interface{}, title string, emptyTableMessage string) error {
	if title != "" {
		fmt.Println(title)
	}

	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)

	if IsTerminal() {
		tableWriter.SetStyle(table.StyleLight)
	}

	tableWriter.Style().Options.SeparateRows = true

	rowsSliceValue := reflect.ValueOf(rows)
	if rowsSliceValue.Len() == 0 && emptyTableMessage != "" {
		PrintMessage(emptyTableMessage)
		return nil
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
		if !columnNameExist && !embedTableExist {
			continue
		}
		embedTableValue := embedTable == "true"

		if embedTableValue {
			var subfieldsProperties []subfieldProperties
			var err error
			columnsNames, columnConfigs, subfieldsProperties, err = appendEmbeddedTableFields(columnsNames, columnConfigs, field)
			if err != nil {
				return err
			}
			fieldsProperties = append(fieldsProperties, fieldProperties{index: i, subfields: subfieldsProperties})
		} else {
			columnsNames = append(columnsNames, columnName)
			fieldsProperties = append(fieldsProperties, fieldProperties{index: i})
			columnMaxWidth, columnMaxWidthExist := field.Tag.Lookup("col-max-width")
			if columnMaxWidthExist {
				columnMaxWidthValue, err := strconv.Atoi(columnMaxWidth)
				if err != nil {
					return errorutils.CheckError(err)
				}
				columnConfigs = append(columnConfigs, table.ColumnConfig{Name: columnName, WidthMax: columnMaxWidthValue})
			}
		}
	}
	tableWriter.AppendHeader(columnsNames)
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

	tableWriter.Render()
	return nil
}

type fieldProperties struct {
	index     int                  // The location of the field inside the row struct
	subfields []subfieldProperties // If this field is an embedded table, this will contain the fields in it
}

type subfieldProperties struct {
	index    int
	maxWidth int
}

func appendEmbeddedTableFields(columnsNames []interface{}, columnConfigs []table.ColumnConfig, field reflect.StructField) ([]interface{}, []table.ColumnConfig, []subfieldProperties, error) {
	rowType := field.Type.Elem()
	fieldsCount := rowType.NumField()
	var subfieldsProperties []subfieldProperties
	for i := 0; i < fieldsCount; i++ {
		innerField := rowType.Field(i)
		columnName, columnNameExist := innerField.Tag.Lookup("col-name")
		if !columnNameExist {
			continue
		}
		columnsNames = append(columnsNames, columnName)
		columnMaxWidth, columnMaxWidthExist := innerField.Tag.Lookup("col-max-width")
		var columnMaxWidthValue int
		var err error
		if columnMaxWidthExist {
			columnMaxWidthValue, err = strconv.Atoi(columnMaxWidth)
			if err != nil {
				return nil, nil, nil, errorutils.CheckError(err)
			}
			columnConfigs = append(columnConfigs, table.ColumnConfig{Name: columnName, WidthMax: columnMaxWidthValue})
		}
		subfieldsProperties = append(subfieldsProperties, subfieldProperties{index: i, maxWidth: columnMaxWidthValue})
	}
	return columnsNames, columnConfigs, subfieldsProperties, nil
}

func appendEmbeddedTableStrings(rowValues []interface{}, fieldValue reflect.Value, subfieldsProperties []subfieldProperties) []interface{} {
	sliceLen := fieldValue.Len()
	numberOfColumns := len(subfieldsProperties)
	tableStrings := make([]string, numberOfColumns)

	for rowIndex := 0; rowIndex < sliceLen; rowIndex++ {
		currRowCells := make([]embeddedTableCell, numberOfColumns)
		maxNumberOfLines := 0

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

// PrintMessage prints message in a frame (which is actaully a table with a single table).
// For example:
// ┌─────────────────────────────────────────┐
// │ An example of a message in a nice frame │
// └─────────────────────────────────────────┘
func PrintMessage(message string) {
	tableWriter := table.NewWriter()
	tableWriter.SetOutputMirror(os.Stdout)
	if IsTerminal() {
		tableWriter.SetStyle(table.StyleLight)
	}
	tableWriter.AppendRow(table.Row{message})
	tableWriter.Render()
}
