package server

import (
	pb "gitlab.com/wjbbig/grpc-stream/proto"
	"sync"
)

type ResponseNotifier struct {
	mutex sync.Mutex
	notifier map[string]chan *pb.ContactMessage
}

func NewResponseNotifier() *ResponseNotifier {
	return &ResponseNotifier{notifier: map[string]chan *pb.ContactMessage{}}
}

func (rn *ResponseNotifier) Add(msgId string) (<-chan *pb.ContactMessage,bool) {
	rn.mutex.Lock()
	defer rn.mutex.Unlock()
	if _, ok := rn.notifier[msgId]; ok {
		return nil, false
	}
	respCh := make(chan *pb.ContactMessage)
	rn.notifier[msgId] = respCh
	return respCh, true
}

func (rn *ResponseNotifier) Delete(msgId string) {
	rn.mutex.Lock()
	defer rn.mutex.Unlock()
	n := rn.notifier[msgId]
	delete(rn.notifier, msgId)

	if n != nil {
		close(n)
	}
}

func (rn *ResponseNotifier) Notify(msgId string) (chan *pb.ContactMessage, bool) {
	if n, ok := rn.notifier[msgId]; ok {
		return n, ok
	}
	return nil, false
}