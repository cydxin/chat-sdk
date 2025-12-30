## WebSocket 消息协议

### 客户端发送消息

```json
{
  "send_to": 1,           // 房间 ID
  "send_type": 1,         // 消息类型：1-文字 2-图片 3-语音
  "send_content": "hello" // 消息内容
}
```

### 服务端推送消息

```json
{
  "id": 123,
  "room_id": 1,
  "sender_id": 1001,
  "msg_type": 1,
  "content": "hello",
  "created_at": 1702345678
}
```

### 服务端通知类型

#### 消息撤回通知
```json
{
  "type": "recall",
  "message_id": 123,
  "room_id": 1,
  "user_id": 1001
}
```

#### 好友申请通知
```json
{
  "type": "friend_request",
  "request_id": 456,
  "from_user": 1001,
  "message": "你好，加个好友吧"
}
```

#### 好友同意通知
```json
{
  "type": "friend_accepted",
  "request_id": 456,
  "user_id": 1002
}
```

#### 成员添加通知
```json
{
  "type": "member_added",
  "room_id": 1,
  "user_id": 1003,
  "operator_id": 1001
}
```