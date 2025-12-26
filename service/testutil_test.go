package service

// 此文件保留作为测试工具入口位（避免破坏包结构）。
// 由于当前 CI/本地环境可能是 CGO_ENABLED=0，使用 sqlite 内存库会失败。
// 具体 mock DB 的实现见 testutil_sqlmock_test.go。
