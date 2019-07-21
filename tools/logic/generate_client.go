package logic

import (
	"fmt"
	"os"
)

func GenerateClient(PD *ProtoDetect, rootDir string) error {
	var clientTemp = `package %s

import (
%s
)

var ServiceName = "%s"
`

	var clientCMDFunTemp = `
func %s(ctx *rpc.Context, req *%s) (*%s, error) {
	rsp := &%s{}
	return rsp, rpc.ClientCall(ctx, ServiceName, %sCMDPath, req, rsp)
}
`

	fn := GetTargetFileName(*PD, "client", rootDir)

	header := ""
	context := ""
	if FileExists(fn) {
		ParseGoCode(PD, fn, "client")
		for i := 0; i < len(PD.RpcList); i++ {
			_, ok := PD.FuncCli[PD.RpcList[i].MethodName]
			if ok == false {
				method := PD.RpcList[i].MethodName
				req := PD.RpcList[i].ReqType
				rsp := PD.RpcList[i].RspType
				cli := fmt.Sprintf(
					clientCMDFunTemp,
					method, req, rsp, rsp, method)
				context += cli
			}
		}
	} else {
		for i := 0; i < len(PD.RpcList); i++ {
			method := PD.RpcList[i].MethodName
			req := PD.RpcList[i].ReqType
			rsp := PD.RpcList[i].RspType
			cli := fmt.Sprintf(
				clientCMDFunTemp,
				method, req, rsp, rsp, method)
			context += cli
		}

		impList := []string{"brick/rpc"}
		impList = append(impList, PD.GetImportPbList()...)
		header = fmt.Sprintf(
			clientTemp, PD.PackageName,
			JoinImportListWithBuf(impList, context), PD.PackageName)
	}

	f, err := os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("can not generate file %s,Error :%v\n", fn, err)

		return err
	}
	if len(header) > 0 {
		if _, err := f.Write([]byte(header)); err != nil {
			return err
		}
	}
	if _, err := f.Write([]byte(context)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}
