package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/hcsshim/internal/trace"
)

func main() {
	provider, err := etw.NewProvider("TraceSample", nil)
	fmt.Println(provider)
	if err != nil {
		os.Exit(1)
	}
	trace.InitWithProvider(provider)

	ctx, _ := trace.NewSpan(context.Background(), "main",
		[]trace.Field{
			{"MainBag1", 37},
			{"MainBag2", 1},
		})
	defer trace.End(ctx, nil)

	foo(ctx)
	bar(ctx)
}

func foo(ctx context.Context) (err error) {
	ctx, span := trace.NewSpan(ctx, "foo",
		[]trace.Field{
			{"FooBag1", "foobar"},
		})
	defer func() { span.End(err) }()

	span.Info("FooEvent", []trace.Field{{"Field1", 66}})

	return errors.New("we failed")
}

func bar(ctx context.Context) {
	ctx, span := trace.NewSpan(ctx, "bar", nil)
	defer span.End(nil)

	doWork(ctx)
}

func doWork(ctx context.Context) {
	trace.Info(ctx, "WorkDone", nil)
	trace.Info(context.Background(), "OrphanEvent", nil)
}
