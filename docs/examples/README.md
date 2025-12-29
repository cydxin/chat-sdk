# Code Examples

This directory contains example code demonstrating various features of the Chat SDK.

## Message UUID Generation

### [message_uuid_demo.go](./message_uuid_demo.go)

Demonstrates the automatic UUID generation feature for `Message.MessageID` introduced in v1.1+.

**Topics covered:**
- Automatic MessageID UUID generation via GORM BeforeCreate hook
- Understanding the difference between `Message.ID` (internal) and `Message.MessageID` (external UUID)
- Using MessageID in API responses
- Retrieving and displaying messages

**Run:**
```bash
# Update the database connection string in the file first
go run -tags ignore message_uuid_demo.go
# OR simply:
go run message_uuid_demo.go
```

## Database Migration

### [migrate_message_id.go](./migrate_message_id.go)

Shows how to migrate the `message_id` column from `bigint` to `VARCHAR(32)` when you have a schema mismatch.

**Use case:**
- Your database has `message_id` as `bigint unsigned`
- Your Go code expects `message_id` as `VARCHAR(32)` for UUIDs
- You need to fix this schema mismatch

**⚠️ WARNING:** This migration will **clear all data** in the `message` table!

**Run:**
```bash
# Update the database connection string in the file first
# BACKUP YOUR DATA BEFORE RUNNING!
go run migrate_message_id.go
```

**Note:** Example files use `// +build ignore` to prevent them from being built during `go test ./...`

## Related Documentation

- [MIGRATION_GUIDE.md](../../MIGRATION_GUIDE.md) - Comprehensive database migration guide
- [README.md](../../README.md) - Main SDK documentation

## Notes

These examples are meant to be run as standalone programs. Copy and adapt them to your needs.

Make sure to:
1. Update the database connection string (DSN)
2. Ensure your database exists and is accessible
3. Backup your data before running any migration
