package server

import "testing"

func TestGenerateMsgId(t *testing.T) {
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
	t.Log(RandomMsgId())
}
