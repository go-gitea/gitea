package base

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
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

func TestEncodeSha256(t *testing.T) {
	assert.Equal(t,
		"c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2",
		EncodeSha256("foobar"),
	)
}

func TestShortSha(t *testing.T) {
	assert.Equal(t, "veryverylo", ShortSha("veryverylong"))
}

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

const gravatarSource = "https://secure.gravatar.com/avatar/"

func disableGravatar() {
	setting.EnableFederatedAvatar = false
	setting.LibravatarService = nil
	setting.DisableGravatar = true
}

func enableGravatar(t *testing.T) {
	setting.DisableGravatar = false
	var err error
	setting.GravatarSourceURL, err = url.Parse(gravatarSource)
	assert.NoError(t, err)
}

func TestSizedAvatarLink(t *testing.T) {
	disableGravatar()
	assert.Equal(t, "/img/avatar_default.png",
		SizedAvatarLink("gitea@example.com", 100))

	enableGravatar(t)
	assert.Equal(t,
		"https://secure.gravatar.com/avatar/353cbad9b58e69c96154ad99f92bedc7?d=identicon&s=100",
		SizedAvatarLink("gitea@example.com", 100),
	)
}

func TestAvatarLink(t *testing.T) {
	disableGravatar()
	assert.Equal(t, "/img/avatar_default.png", AvatarLink("gitea@example.com"))

	enableGravatar(t)
	assert.Equal(t,
		"https://secure.gravatar.com/avatar/353cbad9b58e69c96154ad99f92bedc7?d=identicon",
		AvatarLink("gitea@example.com"),
	)
}

func TestFileSize(t *testing.T) {
	var size int64 = 512
	assert.Equal(t, "512B", FileSize(size))
	size *= 1024
	assert.Equal(t, "512KB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512MB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512GB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512TB", FileSize(size))
	size *= 1024
	assert.Equal(t, "512PB", FileSize(size))
	size *= 4
	assert.Equal(t, "2.0EB", FileSize(size))
}

func TestSubtract(t *testing.T) {
	toFloat64 := func(n interface{}) float64 {
		switch v := n.(type) {
		case int:
			return float64(v)
		case int8:
			return float64(v)
		case int16:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case float32:
			return float64(v)
		case float64:
			return v
		default:
			return 0.0
		}
	}
	values := []interface{}{
		int(-3),
		int8(14),
		int16(81),
		int32(-156),
		int64(1528),
		float32(3.5),
		float64(-15.348),
	}
	for _, left := range values {
		for _, right := range values {
			expected := toFloat64(left) - toFloat64(right)
			sub := Subtract(left, right)
			assert.InDelta(t, expected, sub, 1e-3)
		}
	}
}

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
	testSuccess := func(input []string, expected []int64) {
		result, err := StringsToInt64s(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	}
	testSuccess([]string{}, []int64{})
	testSuccess([]string{"-1234"}, []int64{-1234})
	testSuccess([]string{"1", "4", "16", "64", "256"},
		[]int64{1, 4, 16, 64, 256})

	_, err := StringsToInt64s([]string{"-1", "a", "$"})
	assert.Error(t, err)
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
