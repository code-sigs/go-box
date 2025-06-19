package utils

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GetLocalIP 返回首个可用的非回环 IPv4 地址（跨平台、排除虚拟接口、优先内网 IP）
func GetLocalIP() (string, error) {
	var fallbackIP string // 备用 IP（如 127.0.0.1）

	var virtualPrefixes = []string{
		"docker", "vmnet", "vboxnet", "br-", "veth", "lo", "tun", "tap", // 原有
		"zt", "ham", "npf", "wg", "tailscale", // VPN/Tunnel
		"utun", "macsec", "gpd", // macOS 特有
		"virbr", // Linux 虚拟桥接
	}
	// preferPrefixes := []string{
	// 	"192.168.", "10.", "172.", // 可根据需要调整优先网段
	// }
	preferPrefixes := []string{}
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		// 跳过未启用、回环或无效的接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 排除虚拟网卡
		name := strings.ToLower(iface.Name)
		skip := false
		for _, prefix := range virtualPrefixes {
			if strings.HasPrefix(name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// 遍历接口地址
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil {
				continue // 只取 IPv4
			}

			ipStr := ip.String()

			// 优先返回内网 IP（192.168.x.x, 10.x.x.x）
			for _, pfx := range preferPrefixes {
				if strings.HasPrefix(ipStr, pfx) {
					return ipStr, nil
				}
			}

			// 设置为备选（非优先网段）
			if fallbackIP == "" {
				fallbackIP = ipStr
			}
		}
	}

	// fallback: 返回非优先但有效的 IP（如公网 IP）
	if fallbackIP != "" {
		return fallbackIP, nil
	}

	return "", errors.New("no valid local IP found")
}

// 获取当前时间戳（秒）
func GetTimestamp() int64 {
	return time.Now().Unix()
}

// 获取当前时间戳（毫秒）
func GetMilliTimestamp() int64 {
	return time.Now().UnixNano() / 1e6
}

// 计算字符串 MD5 值
func MD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// 清理字符串（去首尾空格并转小写）
func SanitizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// 生成 UUID v4
func GenerateUUID() string {
	return uuid.New().String()
}

// 生成指定长度的随机字符串（包含字母数字）
func RandString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		result[i] = letters[num.Int64()]
	}
	return string(result)
}

// FormatTime 格式化时间为 "2006-01-02 15:04:05"，若时间为零值则返回空字符串
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// FormatTimeNow 格式化时间为 "2006-01-02 15:04:05"
func FormatTimeNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ParseTime 将字符串按默认格式解析为 *time.Time，失败时返回 nil 和错误
func ParseTime(s string) *time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return &t
	}
	return nil
}

// MillisToTime 将毫秒时间戳转换为 time.Time
func MillisToTime(millis int64) time.Time {
	return time.Unix(millis/1000, (millis%1000)*1e6)
}

// 判断文件或目录是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// 判断字符串是否为纯数字
func IsNumeric(s string) bool {
	match, _ := regexp.MatchString(`^\d+$`, s)
	return match
}

// 获取当前工作目录
func GetCurrentDir() (string, error) {
	return filepath.Abs(".")
}
