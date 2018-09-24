package archwayserver

import (
	"bytes"
	"context"
	"errors"
	"expvar"
	"fmt"
	"time"

	"github.com/Xe/ln"
	"github.com/Xe/ln/opname"
	"github.com/Xe/olin/internal/abi/cwa"
	"github.com/Xe/olin/rpc/archway"
	pubsub "github.com/alash3al/go-pubsub"
	"github.com/pborman/uuid"
	"github.com/perlin-network/life/exec"
)

type Runner struct {
	b *pubsub.Broker
}

func NewRunner(b *pubsub.Broker) *Runner { return &Runner{b: b} }

type handler struct {
	VM         *exec.VirtualMachine
	P          *cwa.Process
	mainFunc   int
	myID       string
	myTopic    string
	cancel     context.CancelFunc
	executions *expvar.Int
}

func (h *handler) handle(ctx context.Context, s *pubsub.Subscriber) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.GetMessages():
			switch msg.GetTopic() {
			case "archway.internal.handler_destroy":
				hd := msg.GetPayload().(*archway.Handler)

				if h.myID == hd.Id {
					h.cancel()
				}
			default:
				runID := uuid.New()
				ev := msg.GetPayload().(*archway.Event)
				env := map[string]string{
					"HANDLER_ID": h.myID,
					"RUN_ID":     runID,
					"EVENT_ID":   ev.Id,
					"TOPIC":      h.myTopic,
				}

				f := ln.F{
					"handler_id":    h.myID,
					"run_id":        runID,
					"event_id":      ev.Id,
					"topic":         h.myTopic,
					"type_of_event": fmt.Sprintf("%T", msg.GetPayload()),
				}

				h.P.Stdin = bytes.NewBuffer(ev.Data)
				h.P.Setenv(env)

				t0 := time.Now()
				ret, err := h.VM.Run(h.mainFunc)
				if err != nil {
					ln.Error(ctx, err, f)
				}
				f["exec_dur"] = time.Since(t0)
				f["ret"] = ret

				ln.Log(ctx, f, ln.Info("VM invocation"))
				h.executions.Add(1)
			}
		}
	}
}

func newHandler(h *archway.Handler) (*handler, error) {
	myID := h.Id

	p := cwa.NewProcess(h.Topic+"+"+myID, []string{"archway", h.Topic}, map[string]string{
		"RUN_ID": myID,
	})

	cfg := exec.VMConfig{
		EnableJIT:          false,
		DefaultMemoryPages: 32, // 2 MB
	}
	vm, err := exec.NewVirtualMachine(h.Module, cfg, p)
	if err != nil {
		return nil, err
	}

	main, ok := vm.GetFunctionExport("cwa_main")
	if !ok {
		return nil, errors.New("archwayserver: need main function to be exported")
	}

	return &handler{
		VM:         vm,
		P:          p,
		mainFunc:   main,
		myID:       myID,
		myTopic:    h.Topic,
		executions: expvar.NewInt(h.Topic + "-" + myID),
	}, nil
}

func (r *Runner) Manage(ctx context.Context) {
	ctx = opname.With(ctx, "Manage")
	cs, err := r.b.Attach()
	if err != nil {
		ln.FatalErr(ctx, err)
	}

	r.b.Subscribe(cs, "archway.internal.handler_create")

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-cs.GetMessages():
			// create handler by id
			hdl := msg.GetPayload().(*archway.Handler)
			vmh, err := newHandler(hdl)
			if err != nil {
				ln.Error(ctx, err, ln.Info("creating new handler from user request"))
				continue
			}

			sb, err := r.b.Attach()
			if err != nil {
				ln.Error(ctx, err, ln.Info("creating subscriber from new handler"))
				continue
			}

			r.b.Subscribe(sb, hdl.Topic, "archway.internal.handler_destroy")
			ctx, cancel := context.WithCancel(context.Background())
			vmh.cancel = cancel
			go vmh.handle(ctx, sb)
		}
	}
}
