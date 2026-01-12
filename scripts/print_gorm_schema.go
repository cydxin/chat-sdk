package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cydxin/chat-sdk/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Usage:
//
//	set CHATSDK_DSN=user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=true&loc=Local
//	go run .\scripts\print_gorm_schema.go
func main() {
	dsn := os.Getenv("CHATSDK_DSN")
	if dsn == "" {
		log.Fatal("CHATSDK_DSN is empty")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&models.Room{}); err != nil {
		log.Fatalf("parse room: %v", err)
	}

	f := stmt.Schema.FieldsByName["RoomID"]
	if f == nil {
		log.Fatalf("RoomID field not found, fields=%v", keysByName(stmt.Schema.FieldsByName))
	}

	// GORM field metadata
	fmt.Println("=== GORM Parsed Field ===")
	fmt.Printf("GoName=%s\n", f.Name)
	fmt.Printf("DBName=%s\n", f.DBName)
	fmt.Printf("DataType=%s\n", f.DataType)
	fmt.Printf("GORMDataType=%s\n", f.GORMDataType)
	fmt.Printf("Size=%d\n", f.Size)
	fmt.Printf("Tag=%s\n", f.Tag.Get("gorm"))

	// Dialect SQL type (what GORM will use in CREATE TABLE / ALTER TABLE)
	sqlType := stmt.DB.Dialector.DataTypeOf(f)
	fmt.Println("=== Dialect SQL Type ===")
	fmt.Println(sqlType)

	// Also show naming strategy table name to avoid table confusion
	ns := schema.NamingStrategy{}
	fmt.Println("=== Table Name (default naming strategy) ===")
	fmt.Println(ns.TableName("room"))

	// Now print actual DB schema
	type col struct {
		Field string
		Type  string
		Null  string
		Key   string
	}
	var cols []col
	// Works on MySQL
	if err := db.Raw("SHOW COLUMNS FROM im_room").Scan(&cols).Error; err != nil {
		fmt.Println("SHOW COLUMNS FROM im_room failed:", err)
		return
	}
	fmt.Println("=== SHOW COLUMNS FROM im_room ===")
	for _, c := range cols {
		fmt.Printf("%s\t%s\t%s\t%s\n", c.Field, c.Type, c.Null, c.Key)
	}
}

func keysByName(m map[string]*schema.Field) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
