package main

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/sijms/go-ora/v2"
	excelize "github.com/xuri/excelize/v2"
)

func getDoc(srn, dir string) (string, error) {
	db, err := sql.Open("oracle", DBConn)
	if err != nil {
		return "", fmt.Errorf("failed to connect database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(query, srn)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	f := excelize.NewFile()
	sheetName := "Sheet1"
	f.SetSheetName(f.GetSheetName(0), sheetName)

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	for i, col := range columns {
		cell := fmt.Sprintf("%c1", 'A'+i)
		if err := f.SetCellValue(sheetName, cell, col); err != nil {
			return "", fmt.Errorf("failed to set header cell value: %w", err)
		}
	}

	rowIndex := 2
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		for i, val := range values {
			cell := fmt.Sprintf("%c%d", 'A'+i, rowIndex)
			cellValue := ""
			if val.Valid {
				cellValue = val.String
			}
			if err := f.SetCellValue(sheetName, cell, cellValue); err != nil {
				return "", fmt.Errorf("failed to set cell value: %w", err)
			}
		}
		rowIndex++
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.xlsx", srn))
	if err := f.SaveAs(filename); err != nil {
		return "", fmt.Errorf("failed to save Excel file: %w", err)
	}

	return filename, nil
}
