package node

import (
	"context"
	"fmt"
)

func (n *nodeImpl) startWorkers(ctx context.Context, num int) {
	for i := 0; i < num; i++ {
		go n.taskWorker(ctx, i+1)
	}
	log.Infof("Started %d workers", num)
}

func (n *nodeImpl) taskWorker(ctx context.Context, id int) {
	log.Start(ctx, fmt.Sprintf("node.taskWorker.%d", id))
	defer log.Finish(ctx)
	for {
		select {
		case nextTask := <-n.taskQueue:
			select {
			case t, ok := <-nextTask:
				if !ok {
				}
				t.Execute()
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
