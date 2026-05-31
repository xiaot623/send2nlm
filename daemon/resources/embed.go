package resources

import "embed"

//go:embed producer/lark.go
//go:embed receiver_lark/lark.go
//go:embed receiver/telegram.go
var Files embed.FS
