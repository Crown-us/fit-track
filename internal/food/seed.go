package food

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed" // Embeds the seed CSV into the binary.
	"encoding/csv"
	"fmt"
	"strconv"
)

//go:embed seed/foods.csv
var seedCSV []byte

// Column positions in seed/foods.csv.
const (
	colName = iota
	colKcal
	colProtein
	colCarbs
	colFat
	colSource
	colStatus
	seedColumnCount
)

// Seed inserts the global foods from the embedded CSV. It is idempotent:
// foods that already exist are left untouched, so it is safe on every start.
func Seed(ctx context.Context, db *sql.DB) error {
	reader := csv.NewReader(bytes.NewReader(seedCSV))
	reader.FieldsPerRecord = seedColumnCount
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("read food seed: %w", err)
	}
	const headerRows = 1
	for line, record := range records[headerRows:] {
		if err := insertSeedRow(ctx, db, record); err != nil {
			return fmt.Errorf("food seed row %d: %w", line+headerRows+1, err)
		}
	}
	return nil
}

func insertSeedRow(ctx context.Context, db *sql.DB, record []string) error {
	values := make([]float64, 0, colSource-colKcal)
	for _, raw := range record[colKcal:colSource] {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("parse number %q: %w", raw, err)
		}
		values = append(values, value)
	}
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO foods (owner_id, name, kcal_per_100g, protein_g_per_100g,
			carbs_g_per_100g, fat_g_per_100g, source, verification_status)
		VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`,
		record[colName], values[0], values[1], values[2], values[3], record[colSource], record[colStatus])
	if err != nil {
		return fmt.Errorf("insert %q: %w", record[colName], err)
	}
	return nil
}
