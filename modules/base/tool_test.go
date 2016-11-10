package base

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
	"strk.kbt.io/projects/go/libravatar"
)

func TestEncodeMD5(t *testing.T) {
	assert.Equal(t,
		"3858f62230ac3c915f300c664312c63f",
		EncodeMD5("foobar"),
	)
}

func TestEncodeSha1(t *testing.T) {
	assert.Equal(t,
		"8843d7f92416211de9ebb963ff4ce28125932878",
		EncodeSha1("foobar"),
	)
}

func TestShortSha(t *testing.T) {
	assert.Equal(t, "veryverylo", ShortSha("veryverylong"))
}

// TODO: Test DetectEncoding()

func TestBasicAuthDecode(t *testing.T) {
	_, _, err := BasicAuthDecode("?")
	assert.Equal(t, "illegal base64 data at input byte 0", err.Error())

	user, pass, err := BasicAuthDecode("Zm9vOmJhcg==")
	assert.NoError(t, err)
	assert.Equal(t, "foo", user)
	assert.Equal(t, "bar", pass)
}

func TestBasicAuthEncode(t *testing.T) {
	assert.Equal(t, "Zm9vOmJhcg==", BasicAuthEncode("foo", "bar"))
}

func TestGetRandomString(t *testing.T) {
	assert.Len(t, GetRandomString(4), 4)
}

// TODO: Test PBKDF2()
// TODO: Test VerifyTimeLimitCode()
// TODO: Test CreateTimeLimitCode()

func TestHashEmail(t *testing.T) {
	assert.Equal(t,
		"d41d8cd98f00b204e9800998ecf8427e",
		HashEmail(""),
	)
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		HashEmail("gitea@example.com"),
	)
}

func TestAvatarLink(t *testing.T) {
	setting.EnableFederatedAvatar = false
	setting.LibravatarService = nil
	setting.DisableGravatar = true

	assert.Equal(t, "/img/avatar_default.png", AvatarLink(""))

	setting.DisableGravatar = false
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		AvatarLink("gitea@example.com"),
	)

	setting.EnableFederatedAvatar = true
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		AvatarLink("gitea@example.com"),
	)
	setting.LibravatarService = libravatar.New()
	assert.Equal(t,
		"http://cdn.libravatar.org/avatar/353cbad9b58e69c96154ad99f92bedc7",
		AvatarLink("gitea@example.com"),
	)
}

// TODO: computeTimeDiff()
// TODO: TimeSincePro()
// TODO: timeSince()
// TODO: RawTimeSince()
// TODO: TimeSince()

func TestFileSize(t *testing.T) {
	var size int64
	size = 512
	assert.Equal(t, "512B", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512KB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512MB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512GB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512TB", FileSize(size))
	size = size * 1024
	assert.Equal(t, "512PB", FileSize(size))
	//size = size * 1024 TODO: Fix bug for EB
	//assert.Equal(t, "512EB", FileSize(size))
}

// TODO: Subtract()

func TestEllipsisString(t *testing.T) {
	assert.Equal(t, "...", EllipsisString("foobar", 0))
	assert.Equal(t, "...", EllipsisString("foobar", 1))
	assert.Equal(t, "...", EllipsisString("foobar", 2))
	assert.Equal(t, "...", EllipsisString("foobar", 3))
	assert.Equal(t, "f...", EllipsisString("foobar", 4))
	assert.Equal(t, "fo...", EllipsisString("foobar", 5))
	assert.Equal(t, "foobar", EllipsisString("foobar", 6))
	assert.Equal(t, "foobar", EllipsisString("foobar", 10))
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "", TruncateString("foobar", 0))
	assert.Equal(t, "f", TruncateString("foobar", 1))
	assert.Equal(t, "fo", TruncateString("foobar", 2))
	assert.Equal(t, "foo", TruncateString("foobar", 3))
	assert.Equal(t, "foob", TruncateString("foobar", 4))
	assert.Equal(t, "fooba", TruncateString("foobar", 5))
	assert.Equal(t, "foobar", TruncateString("foobar", 6))
	assert.Equal(t, "foobar", TruncateString("foobar", 7))
}

func TestStringsToInt64s(t *testing.T) {
	assert.Equal(t, []int64{}, StringsToInt64s([]string{}))
	assert.Equal(t,
		[]int64{1, 4, 16, 64, 256},
		StringsToInt64s([]string{"1", "4", "16", "64", "256"}),
	)

	// TODO: StringsToInt64s should return ([]int64, error)
	assert.Equal(t, []int64{-1, 0, 0}, StringsToInt64s([]string{"-1", "a", "$"}))
}

func TestInt64sToStrings(t *testing.T) {
	assert.Equal(t, []string{}, Int64sToStrings([]int64{}))
	assert.Equal(t,
		[]string{"1", "4", "16", "64", "256"},
		Int64sToStrings([]int64{1, 4, 16, 64, 256}),
	)
}

func TestInt64sToMap(t *testing.T) {
	assert.Equal(t, map[int64]bool{}, Int64sToMap([]int64{}))
	assert.Equal(t,
		map[int64]bool{1: true, 4: true, 16: true},
		Int64sToMap([]int64{1, 4, 16}),
	)
}

func TestIsLetter(t *testing.T) {
	assert.True(t, IsLetter('a'))
	assert.True(t, IsLetter('e'))
	assert.True(t, IsLetter('q'))
	assert.True(t, IsLetter('z'))
	assert.True(t, IsLetter('A'))
	assert.True(t, IsLetter('E'))
	assert.True(t, IsLetter('Q'))
	assert.True(t, IsLetter('Z'))
	assert.True(t, IsLetter('_'))
	assert.False(t, IsLetter('-'))
	assert.False(t, IsLetter('1'))
	assert.False(t, IsLetter('$'))
}

func TestIsTextFile(t *testing.T) {
	assert.True(t, IsTextFile([]byte{}))
	assert.True(t, IsTextFile([]byte("lorem ipsum")))
}

// TODO: IsImageFile(), currently no idea how to test
// TODO: IsPDFFile(), currently no idea how to test
