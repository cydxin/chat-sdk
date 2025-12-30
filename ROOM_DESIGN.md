# 房间设计说明

## 房间ID设计

### 1. 私聊房间
- **RoomID 格式**: `private_{小UserID}_{大UserID}`
- **示例**: `private_1001_1002`
- **生成规则**: 
  - 将两个用户ID排序（小的在前）
  - 使用固定格式拼接
  - 保证两个用户之间只有一个私聊房间

### 2. 群聊房间
- **RoomID 格式**: `group_{短UUID}`
- **示例**: `group_a1b2c3d4`
- **生成规则**:
  - 使用 UUID 前8位
  - 保证唯一性和安全性

## 数据库设计

### Room 表
- `ID`: 数据库主键（自增uint64），**用于内部关联**
- `RoomID`: 对外展示的房间号（string），**用于前端/API交互**

### RoomUser 表（已存在）
- 记录用户和房间的关系
- `room_id` 关联到 `Room.ID`（数字）
- `user_id` 关联到 `User.ID`

## 消息发送流程

```
1. 前端发送消息
   {
     "send_to": "private_1001_1002",  // 或 "group_a1b2c3d4"
     "send_content": "Hello",
     "send_type": 1
   }

2. 后端接收处理
   - 通过 RoomID (字符串) 查询 Room
   - 使用 Room.ID (数字) 保存消息
   - 通过 RoomUser 表查询房间成员
   - 推送消息给所有成员

3. 消息推送响应
   {
     "id": 123,
     "room_id": "private_1001_1002",  // 返回字符串 RoomID
     "sender_id": 1001,
     "msg_type": 1,
     "content": "Hello",
     "created_at": "2025-01-29T10:00:00Z"
   }
```

## 好友申请通过后的处理

当用户同意好友申请时，系统自动：

1. ✅ 创建双向好友关系（Friend 表）
2. ✅ 创建私聊房间（使用 `private_{小ID}_{大ID}` 格式）
3. ✅ 添加两个用户到 RoomUser 表

**代码位置**: `service/member_service.go` -> `AcceptFriendRequest`

## 核心方法

### RoomService

- `GetRoomByRoomID(roomID string)`: 根据字符串 RoomID 查询房间
- `CreatePrivateRoom(user1, user2)`: 创建或获取私聊房间
- `CreateGroupRoom(name, creator, members)`: 创建群聊房间
- `GetRoomMembers(roomID uint64)`: 获取房间成员列表

### MemberService

- `AcceptFriendRequest(requestID, userID)`: 同意好友申请（自动创建私聊房间）
- `generatePrivateRoomID(userID1, userID2)`: 生成私聊房间ID

## 优势

1. **私聊和群聊统一处理**: 都是房间，逻辑一致
2. **RoomID 可读性**: 私聊房间可以直接看出参与者
3. **安全性**: 不暴露内部数字ID
4. **性能**: 使用数字ID做数据库关联，高效
5. **查询方便**: 私聊房间使用固定规则，避免重复创建

