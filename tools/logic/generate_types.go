package logic

import (
	"brick/log"
	"bytes"
	"fmt"
	"github.com/emicklei/proto"
	"os"
	"os/exec"
	"strings"
)

func GenerateTypes(pbFileName string, outDir string, incPaths []string) error {
	target := fmt.Sprintf(
		"--tstypes_out=int_enums=true,original_names=true,int64_string=true:%s", outDir)

	log.Infof("** ts out dir %s", outDir)

	var args []string
	for _, x := range incPaths {
		args = append(args, fmt.Sprintf("-I=%s", x))
	}
	args = append(args, target, pbFileName)

	cmd := exec.Command("protoc", args...)

	log.Infof("protoc args %v", args)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		log.Fatal("create typescript file error ", cmd.Stderr)
		return err
	}

	outStr := outBuf.String()
	errStr := errBuf.String()

	if outStr != "" {
		log.Info("out:", outStr)
	}
	if errStr != "" {
		log.Error("err:", errStr)
	}

	return nil
}

type TsContext struct {
	enumList []*proto.Enum
	msgList  []*proto.Message

	addr2Msg map[string]*proto.Message
}

func (p *TsContext) GetParent(v proto.Visitee) string {
	var nameList []string
	for v != nil {
		m := p.addr2Msg[fmt.Sprintf("%p", v)]
		if m == nil {
			break
		}
		nameList = append(nameList, m.Name)

		vv := &ProtoVisitor4Ts{}
		v.Accept(vv)

		if len(vv.msgList) == len(p.msgList) || len(vv.msgList) != 1 {
			// 到顶层了
			break
		}

		x := vv.msgList[0]

		v = x.Parent
	}

	if len(nameList) == 0 {
		return ""
	} else if len(nameList) == 1 {
		return nameList[0]
	}

	var rev []string
	for i := len(nameList) - 1; i >= 0; i-- {
		rev = append(rev, nameList[i])
	}

	return strings.Join(rev, ".")
}

func isBuiltInType(typ string) bool {
	switch typ {
	case "string", "uint32", "int32", "uint64", "int64", "bool", "bytes", "float", "double":
		return true
	}
	return false
}

func (p *TsContext) GetParentWithType(typ string, v proto.Visitee) string {
	if isBuiltInType(typ) {
		return ""
	}

	// search up
	for v != nil {
		m := p.addr2Msg[fmt.Sprintf("%p", v)]
		if m == nil {
			break
		}

		for _, y := range m.Elements {
			vv := &ProtoVisitor4Ts{}
			y.Accept(vv)
			for _, x := range vv.msgList {
				if x.Name == typ {
					goto OUT
				}
			}
		}

		v = m.Parent
	}
OUT:

	var nameList []string
	for v != nil {
		m := p.addr2Msg[fmt.Sprintf("%p", v)]
		if m == nil {
			break
		}
		nameList = append(nameList, m.Name)

		vv := &ProtoVisitor4Ts{}
		v.Accept(vv)

		if len(vv.msgList) != 1 {
			break
		}

		x := vv.msgList[0]

		v = x.Parent
	}

	if len(nameList) == 0 {
		return ""
	} else if len(nameList) == 1 {
		return nameList[0]
	}

	var rev []string
	for i := len(nameList) - 1; i >= 0; i-- {
		rev = append(rev, nameList[i])
	}

	return strings.Join(rev, ".")
}

type tsWriter struct {
	i  int
	fp *os.File
}

func (p *tsWriter) open(path string) error {
	if p.fp != nil {
		p.fp.Close()
		p.fp = nil
	}

	var err error
	p.fp, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Errorf("create ts file %s err %v", path, err)
		return err
	}

	return nil
}

func (p *tsWriter) incIndent() {
	p.i++
}

func (p *tsWriter) decIndent() {
	if p.i > 0 {
		p.i--
	}
}

func (p *tsWriter) out(template string, args ...interface{}) {
	if !strings.HasSuffix(template, "\n") {
		template += "\n"
	}
	s := fmt.Sprintf(template, args...)

	if template != "\n" {
		for i := 0; i < p.i; i++ {
			p.fp.WriteString("    ")
		}
	}
	p.fp.WriteString(s)
}

var W = &tsWriter{}

func (p *TsContext) GetEnumFullName(e *proto.Enum) string {
	parent := p.GetParent(e.Parent)

	var fullName string
	if parent != "" {
		fullName = fmt.Sprintf("%s.%s", parent, e.Name)
	} else {
		fullName = e.Name
	}
	return fullName
}

func (p *TsContext) GetMsgFullName(e *proto.Message) string {
	parent := p.GetParent(e.Parent)

	var fullName string
	if parent != "" {
		fullName = fmt.Sprintf("%s.%s", parent, e.Name)
	} else {
		fullName = e.Name
	}
	return fullName
}

type ProtoVisitor4Ts struct {
	msgList      []*proto.Message
	EnumFields   []*proto.EnumField
	normalFields []*proto.NormalField
	mapFields    []*proto.MapField
}

func (p *ProtoVisitor4Ts) VisitMessage(m *proto.Message) {
	p.msgList = append(p.msgList, m)
}

func (p *ProtoVisitor4Ts) VisitService(v *proto.Service) {
}

func (p *ProtoVisitor4Ts) VisitSyntax(s *proto.Syntax) {
}

func (p *ProtoVisitor4Ts) VisitPackage(pkg *proto.Package) {
}

func (p *ProtoVisitor4Ts) VisitOption(o *proto.Option) {
}

func (p *ProtoVisitor4Ts) VisitImport(i *proto.Import) {
}

func (p *ProtoVisitor4Ts) VisitNormalField(i *proto.NormalField) {
	p.normalFields = append(p.normalFields, i)
}

func (p *ProtoVisitor4Ts) VisitEnumField(i *proto.EnumField) {
	p.EnumFields = append(p.EnumFields, i)
}

func (p *ProtoVisitor4Ts) VisitEnum(e *proto.Enum) {
}

func (p *ProtoVisitor4Ts) VisitComment(e *proto.Comment) {
}

func (p *ProtoVisitor4Ts) VisitOneof(o *proto.Oneof) {
}

func (p *ProtoVisitor4Ts) VisitOneofField(o *proto.OneOfField) {
}

func (p *ProtoVisitor4Ts) VisitReserved(rs *proto.Reserved) {
}

func (p *ProtoVisitor4Ts) VisitRPC(rpc *proto.RPC) {
}

func (p *ProtoVisitor4Ts) VisitMapField(f *proto.MapField) {
	p.mapFields = append(p.mapFields, f)
}

func (p *ProtoVisitor4Ts) VisitGroup(g *proto.Group) {
}
func (p *ProtoVisitor4Ts) VisitExtensions(e *proto.Extensions) {
}
func (p *ProtoVisitor4Ts) VisitProto(*proto.Proto) {
}

func walkPb4Ts(pd *ProtoDetect, definition *proto.Proto, ctx *TsContext) {
	handleEnum := func(e *proto.Enum) {
		ctx.enumList = append(ctx.enumList, e)
	}

	handleMsg := func(p *proto.Message) {
		ctx.msgList = append(ctx.msgList, p)
		ctx.addr2Msg[fmt.Sprintf("%p", p)] = p
	}

	proto.Walk(
		definition,
		proto.WithEnum(handleEnum),
		proto.WithMessage(handleMsg),
	)
}

func getTsType(typ string) string {
	var jt = typ

	switch typ {
	case "string", "uint64", "int64", "bytes":
		jt = "string"
	case "uint32", "int32":
		jt = "number"
	case "float", "double":
		jt = "number"
	case "bool":
		jt = "boolean"
	}

	return jt
}

func parsePb4Ts(pd *ProtoDetect, protoFile string) *TsContext {
	reader, err := os.Open(protoFile)
	if err != nil {
		log.Fatalf("can not open proto file %s, error is %v", protoFile, err)
	}
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, err := parser.Parse()
	if err != nil {
		log.Fatalf("proto parse error %v", err)
	}

	ctx := &TsContext{addr2Msg: make(map[string]*proto.Message)}
	walkPb4Ts(pd, definition, ctx)

	return ctx
}

func GenerateTs(pd *ProtoDetect, protoFile string, outDir string) error {
	ctx := parsePb4Ts(pd, protoFile)

	if outDir == "" {
		outDir = "."
	}

	err := W.open(fmt.Sprintf("%s/%s.%s.d.ts", outDir, pd.SvrName, pd.SvrName))
	if err != nil {
		return err
	}

	W.out("// Code generated by rpc_gen. DO NOT EDIT.")
	W.out("")

	W.out("declare namespace %s {", pd.SvrName)

	// 输出枚举
	W.incIndent()
	for _, v := range ctx.enumList {
		fullName := ctx.GetEnumFullName(v)
		fullName = strings.Replace(fullName, ".", "_", -1)

		W.out("export const enum %s {", fullName)
		W.incIndent()

		var pv ProtoVisitor4Ts
		for _, ei := range v.Elements {
			ei.Accept(&pv)
		}
		for _, x := range pv.EnumFields {
			if x.Comment != nil {
				for _, y := range x.Comment.Lines {
					W.out("//%s", y)
				}
			}

			if x.InlineComment != nil && len(x.InlineComment.Lines) > 0 {
				W.out("%s = %d, //%s", x.Name, x.Integer, x.InlineComment.Lines[0])
			} else {
				W.out("%s = %d,", x.Name, x.Integer)
			}
		}

		W.decIndent()
		W.out("}")
		W.out("")
	}
	W.decIndent()

	allPb := make(map[string]bool)
	for _, v := range ctx.msgList {
		fullName := ctx.GetMsgFullName(v)
		fullName = strings.Replace(fullName, ".", "_", -1)
		allPb[fullName] = true
	}

	// 输出 message
	W.incIndent()
	for _, v := range ctx.msgList {
		//export interface IdItem {
		//  id?: string;
		//  name?: string;
		//  type?: number;
		//}

		fullName := ctx.GetMsgFullName(v)
		fullName = strings.Replace(fullName, ".", "_", -1)

		W.out("export interface %s {", fullName)
		W.incIndent()
		{
			var pv ProtoVisitor4Ts
			for _, ei := range v.Elements {
				ei.Accept(&pv)
			}

			first := true
			for _, x := range pv.normalFields {
				if x.Comment != nil && len(x.Comment.Lines) > 0 {
					if !first {
						W.out("")
					}
					for _, y := range x.Comment.Lines {
						W.out("//%s", y)
					}
				}

				typ := x.Type
				dot := strings.Index(typ, ".")
				if dot < 0 {
					parent := ctx.GetParentWithType(typ, x.Parent)
					if parent != "" {
						tmp := fmt.Sprintf("%s.%s", parent, typ)
						tmp = strings.Replace(tmp, ".", "_", -1)

						if allPb[tmp] {
							typ = tmp
						}
					}
				}

				var ic string
				if x.InlineComment != nil && len(x.InlineComment.Lines) > 0 {
					ic = x.InlineComment.Lines[0]
				}
				if ic != "" {
					ic = " //" + ic
				}

				// array?
				if x.Repeated {
					W.out("%s?: Array<%s>;%s", x.Name, getTsType(typ), ic)
				} else {
					W.out("%s?: %s;%s", x.Name, getTsType(typ), ic)
				}

				if first {
					first = false
				}
			}

			for _, x := range pv.mapFields {
				if x.Comment != nil && len(x.Comment.Lines) > 0 {
					if !first {
						W.out("")
					}
					for _, y := range x.Comment.Lines {
						W.out("//%s", y)
					}
				}

				typ := x.Type
				dot := strings.Index(typ, ".")
				if dot < 0 {
					parent := ctx.GetParentWithType(typ, x.Parent)
					if parent != "" {
						tmp := fmt.Sprintf("%s.%s", parent, typ)
						tmp = strings.Replace(tmp, ".", "_", -1)

						if allPb[tmp] {
							typ = tmp
						}
					}
				}

				var ic string
				if x.InlineComment != nil && len(x.InlineComment.Lines) > 0 {
					ic = x.InlineComment.Lines[0]
				}
				if ic != "" {
					ic = " //" + ic
				}

				W.out("%s?: {[key: %s]: %s};%s",
					x.Name, getTsType(x.KeyType), getTsType(typ), ic)

				if first {
					first = false
				}
			}
		}
		W.decIndent()
		W.out("}")
		W.out("")
	}
	W.decIndent()

	// 输出 rpc
	W.incIndent()
	W.out("export interface %sService {", pd.SvrName)
	W.incIndent()
	{
		first := true
		for _, v := range pd.RpcList {
			if len(v.CommentLines) > 0 {
				if !first {
					W.out("")
				}
				for _, y := range v.CommentLines {
					W.out("//%s", y)
				}
			}

			W.out("%s: (r:%s) => %s;", v.MethodName, v.ReqType, v.RspType)

			if first {
				first = false
			}
		}
	}
	W.decIndent()
	W.out("}")
	W.decIndent()

	W.out("}")

	W.fp.Close()
	W.fp = nil

	return nil
}
