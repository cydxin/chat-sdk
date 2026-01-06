package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestVerifyCodeService_SendAndVerify(t *testing.T) {
	// 注意：这里测试的是 VerifyCodeService 的行为（它会返回 code），
	// 对外 API 是否返回 code 由 handler 按 Config.Service.Debug 控制。
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewVerifyCodeService(rdb)
	ctx := context.Background()

	ret, err := svc.SendCode(ctx, VerifyCodePurposeRegister, "13800138000")
	if err != nil {
		t.Fatalf("SendCode err: %v", err)
	}
	if ret == nil || ret.Code == "" {
		t.Fatalf("expected code, got %#v", ret)
	}

	ok, err := svc.VerifyCode(ctx, VerifyCodePurposeRegister, "13800138000", ret.Code)
	if err != nil {
		t.Fatalf("VerifyCode err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok")
	}

	// once-only
	ok, err = svc.VerifyCode(ctx, VerifyCodePurposeRegister, "13800138000", ret.Code)
	if err != nil {
		t.Fatalf("VerifyCode second err: %v", err)
	}
	if ok {
		t.Fatalf("expected not ok after delete")
	}
}

func TestVerifyCodeService_Cooldown(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewVerifyCodeService(rdb)
	svc.cooldown = 2 * time.Second
	ctx := context.Background()

	ret1, err := svc.SendCode(ctx, VerifyCodePurposeRegister, "a@b.com")
	if err != nil {
		t.Fatalf("SendCode 1 err: %v", err)
	}
	if ret1.Code == "" {
		t.Fatalf("expected code")
	}

	ret2, err := svc.SendCode(ctx, VerifyCodePurposeRegister, "A@B.COM")
	if err != nil {
		t.Fatalf("SendCode 2 err: %v", err)
	}
	// cooldown 时不返回 code
	if ret2.Code != "" {
		t.Fatalf("expected empty code due to cooldown")
	}
}
