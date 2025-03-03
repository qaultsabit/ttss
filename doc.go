package main

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/sijms/go-ora/v2"
	excelize "github.com/xuri/excelize/v2"
)

func getDoc(srn, dir string) error {
	db, err := sql.Open("oracle", DBConn)
	if err != nil {
		return fmt.Errorf("failed to connect database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(query, srn)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	f := excelize.NewFile()
	sheetName := "Sheet1"
	f.SetSheetName(f.GetSheetName(0), sheetName)

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %v", err)
	}

	for i, col := range columns {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		f.SetCellValue(sheetName, cell, col)
	}

	rowIndex := 2
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("failed to scan row: %v", err)
		}

		for i, val := range values {
			cell := fmt.Sprintf("%s%d", string(rune('A'+i)), rowIndex)
			if err := f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", val)); err != nil {
				return fmt.Errorf("failed to set cell value: %v", err)
			}
		}
		rowIndex++
	}

	filePath := filepath.Join(dir, "doc.xlsx")
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save Excel file: %v", err)
	}

	return nil
}
