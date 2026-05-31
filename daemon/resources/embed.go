package resources

import "embed"

//go:embed producer/lark.go
//go:embed producer/arxiv/arxiv.go
//go:embed producer/hf_paper/hf_paper.go
//go:embed receiver_lark/lark.go
//go:embed producer/weixin/weixin.go
//go:embed receiver/telegram.go
var Files embed.FS
