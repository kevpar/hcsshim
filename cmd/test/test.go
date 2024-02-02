package main

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/save"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func main() {
	r := &save.StartSaveResponse{
		Resources: []*save.SaveResource{
			{
				Id: "e4445f92-71ee-4e95-a59a-2b54815ad851",
				Data: &save.SaveResource_Rootfs{
					Rootfs: &save.SaveRootFS{
						ContainerId: "301571993da654d82d67c0b09d0fbc54efc07cf3d3c09f537bd649e9412c6e23",
					},
				},
			},
		},
	}

	pjson, err := protojson.MarshalOptions{}.Marshal(r)
	if err != nil {
		panic(err)
	}
	pwire, err := proto.MarshalOptions{}.Marshal(r)
	if err != nil {
		panic(err)
	}
	fmt.Printf("PSJON:\n%s\n", pjson)
	fmt.Printf("\n")
	fmt.Printf("PWIRE:\n%x\n", pwire)
}
