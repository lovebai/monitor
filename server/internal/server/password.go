package server

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	passwordHashPrefix = "pbkdf2$"
	passwordIterations = 210000
	passwordKeyLen     = 32
	passwordSaltLen    = 16
)

// GeneratePasswordHash 使用 PBKDF2-HMAC-SHA256 生成带随机盐的登录密码哈希，
// 格式为 pbkdf2$sha256$<迭代次数>$<盐 base64>$<哈希 base64>，可直接写入 auth_password。
func GeneratePasswordHash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	salt := make([]byte, passwordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%ssha256$%d$%s$%s", passwordHashPrefix, passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// IsPasswordHash 判断配置中的密码是否已是加密哈希（而非明文）。
func IsPasswordHash(s string) bool {
	return strings.HasPrefix(s, passwordHashPrefix)
}

// VerifyPasswordHash 校验明文密码是否与加密哈希匹配；非本格式返回 false。
func VerifyPasswordHash(password, encoded string) bool {
	if !IsPasswordHash(encoded) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, passwordHashPrefix), "$")
	if len(parts) != 4 || parts[0] != "sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// UpdateConfigPassword 将配置文件中的 auth_password 行替换为加密哈希；
// 未找到该行时追加到文件末尾。行首缩进与其余内容保持不变。
func UpdateConfigPassword(path, hash string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	lines := strings.Split(string(b), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "auth_password:") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "auth_password: \"" + hash + "\""
		replaced = true
		break
	}
	if !replaced {
		lines = append(lines, "auth_password: \""+hash+"\"")
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
