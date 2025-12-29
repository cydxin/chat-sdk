# Database Migration Guide

## Message.MessageID Schema Migration

### Background

The `Message` struct in `models/table.go` defines `MessageID` as a `VARCHAR(32)` field for storing UUIDs:

```go
type Message struct {
    ID        uint64 `gorm:"primarykey"`              // Internal database primary key
    MessageID string `gorm:"size:32;uniqueIndex"`     // External UUID for API responses
    // ...
}
```

However, if your database was created with an older schema or before this field was properly defined, the `message_id` column might be `BIGINT` instead of `VARCHAR(32)`.

### How to Check if You Need Migration

Run this SQL query to check the column type:

```sql
SHOW COLUMNS FROM im_message LIKE 'message_id';
```

If the `Type` column shows `bigint` or similar numeric type, you need to run the migration.

### Running the Migration

#### Option 1: Using the Migration Function (Recommended)

After initializing the ChatEngine, call the migration function:

```go
package main

import (
    "log"
    chat "github.com/cydxin/chat-sdk"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func main() {
    // Initialize database
    db, err := gorm.Open(mysql.Open("user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4"), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // Create engine
    engine := chat.NewEngine(
        chat.WithDB(db),
        chat.WithTablePrefix("im_"),
    )

    // Run migration
    if err := engine.MigrateMessageIDToUUID(); err != nil {
        log.Fatalf("Migration failed: %v", err)
    }

    log.Println("Migration completed successfully!")
}
```

**⚠️ WARNING**: This migration will **clear all data** in the `message` table because bigint IDs cannot be directly converted to UUIDs. Make sure to backup your data first!

#### Option 2: Manual SQL Migration

If you prefer to run SQL manually or need to preserve data:

```sql
-- 1. Backup the message table
CREATE TABLE im_message_backup AS SELECT * FROM im_message;

-- 2. Truncate the table
TRUNCATE TABLE im_message;

-- 3. Alter the column type
ALTER TABLE im_message MODIFY COLUMN message_id VARCHAR(32) NOT NULL;

-- 4. Ensure unique index exists
CREATE UNIQUE INDEX idx_message_message_id ON im_message(message_id);
```

### After Migration

After running the migration:

1. The `message_id` column will be `VARCHAR(32)`
2. New messages will automatically get a UUID assigned via the `BeforeCreate` hook
3. The schema will match the Go struct definition

### Understanding Message IDs

The `Message` struct has two ID fields:

- **`ID` (uint64)**: Internal database primary key, used for foreign key references within the database
- **`MessageID` (string)**: External UUID, used in API responses and for client-side message identification

Foreign key references in other tables (like `MessageStatus.MessageID`, `Conversation.LastMessageID`) refer to the internal `ID` field, not the UUID `MessageID` field.

### Testing After Migration

```go
// Test message creation
msg, err := engine.MsgService.SaveMessage(roomID, senderID, "Hello", 1)
if err != nil {
    log.Fatal(err)
}

// Verify MessageID is a UUID
log.Printf("Message ID: %s (should be UUID format)", msg.MessageID)
log.Printf("Internal ID: %d", msg.ID)
```

### Rollback

If you need to rollback (not recommended):

```sql
-- Restore from backup
DROP TABLE im_message;
CREATE TABLE im_message AS SELECT * FROM im_message_backup;
```

## Automatic UUID Generation

With the migration complete, the system now automatically generates UUIDs for messages:

```go
// In models/table.go
func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
    if m.MessageID == "" {
        m.MessageID = uuid.New().String()
    }
    return nil
}
```

This ensures consistency with other UUID fields in the system:
- `User.UID` - User UUID
- `Room.RoomID` - Room UUID  
- `Message.MessageID` - Message UUID

## Need Help?

If you encounter any issues during migration:
1. Check that your database user has `ALTER TABLE` permissions
2. Ensure you're using the correct table prefix in your configuration
3. Backup your data before running any migration
4. Check the logs for detailed error messages
