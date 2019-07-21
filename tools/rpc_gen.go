package main

import (
	"brick/log"
	"brick/rpc"
	"brick/tools/codegen"
	"brick/tools/rpc_gen/logic"
	"brick/tools/tools_builder/tools_lib"
	"brick/utils"
	"bytes"
	"fmt"
	"github.com/emicklei/proto"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var PD *logic.ProtoDetect
var sep string

func init() {
	if runtime.GOOS == "windows" {
		sep = `\`
	} else {
		sep = "/"
	}
}

var pbImportParsed = make(map[string]bool)
var pbList []*logic.PbMsg

func walkPb(definition *proto.Proto, pd *logic.ProtoDetect) {
	handlePackage := func(p *proto.Package) {
		pd.PackageName = p.Name
	}

	handleService := func(s *proto.Service) {
		pd.SvrName = s.Name
	}

	handleOption := func(o *proto.Option) {
		if o.Name == "go_package" {
			pd.GoPackageName = o.Constant.Source
		}
	}

	handleRpc := func(m *proto.RPC) {
		cmdID := 0
		url := ""
		flags := 0
		for _, opt := range m.Elements {
			v := &logic.ProtoVisitor{}
			opt.Accept(v)
			if v.CmdID > 0 {
				cmdID = int(v.CmdID)
			}
			if v.Url != "" {
				url = v.Url
			}
			if v.Flags > 0 {
				flags = int(v.Flags)
			}
		}

		//if cmdID == 0 {
		//	log.Fatalf("method `%s` missed CmdID option", m.Name)
		//}

		node := &logic.RpcNode{
			MethodName: m.Name,
			ReqType:    m.RequestType,
			RspType:    m.ReturnsType,
			CmdID:      strconv.Itoa(cmdID),
			Url:        url,
			Flags:      strconv.Itoa(flags),
		}

		if m.Comment != nil {
			node.CommentLines = m.Comment.Lines
		}

		pd.RpcList = append(
			pd.RpcList,
			node)
	}

	handleEnum := func(e *proto.Enum) {
		if strings.HasSuffix(e.Name, "ErrCode") {
			var pv logic.ProtoVisitor
			for _, ei := range e.Elements {
				ei.Accept(&pv)
			}
			pd.ErrCodes = append(
				pd.ErrCodes,
				logic.ErrCodeDef{ErrCodeSetName: e.Name, ErrCodeEnums: pv.EnumFields})
		}
	}

	handleImport := func(i *proto.Import) {
		if pbImportParsed[i.Filename] {
			return
		}
		defer func() {
			pbImportParsed[i.Filename] = true
		}()

		if strings.HasPrefix(i.Filename, "google/") {
			return
		}

		pb := logic.SearchImportPb(i.Filename)
		if pb != "" {
			old := logic.CurrentPb

			logic.SetCurrentPb(i.Filename)
			pdImport := parsePbOrDie(pb)

			if pdImport.GoPackageName != "" {
				pd.ImportList = append(
					pd.ImportList, &logic.ImportNode{
						ImportPath: i.Filename, GoPackage: pdImport.GoPackageName})
			}

			logic.SetCurrentPb(old)
		} else {
			log.Fatalf("not found %s", i.Filename)
		}
	}

	handleMsg := func(p *proto.Message) {
		pbMsg := &logic.PbMsg{Name: p.Name, ModName: logic.CurrentMod}
		vv := &logic.ProtoVisitor{CurMsg: pbMsg}
		for _, v := range p.Elements {
			v.Accept(vv)
		}
		pbList = append(pbList, pbMsg)

		key := fmt.Sprintf("%s_%s", logic.CurrentMod, p.Name)

		if e, ok := logic.PbMap[key]; ok {
			e.NameDupCnt++
		} else {
			logic.PbMap[key] = pbMsg
		}
	}

	proto.Walk(
		definition,
		proto.WithService(handleService),
		proto.WithPackage(handlePackage),
		proto.WithOption(handleOption),
		proto.WithRPC(handleRpc),
		proto.WithEnum(handleEnum),
		proto.WithImport(handleImport),
		proto.WithMessage(handleMsg),
	)
}

func getIncludePathList(pbFilePath string) []string {
	var incPaths []string

	i := strings.LastIndex(pbFilePath, "/")
	if i > 0 {
		incPaths = append(incPaths, pbFilePath[:i])
	}
	incPaths = append(incPaths, ".")

	goPath := os.Getenv("GOPATH")
	if goPath != "" {
		var s string
		if runtime.GOOS == "windows" {
			s = ";"
		} else {
			s = ":"
		}
		fs := strings.Split(goPath, s)
		for _, f := range fs {
			if f != "" {
				p := fmt.Sprintf("%s%sproto", f, sep)
				if logic.FileExists(p) {
					incPaths = append(incPaths, p)
				}
			}
		}
	}

	extraList := tools_lib.OptStrDef("I", "")
	if extraList != "" {
		fs := strings.Split(extraList, ",")
		incPaths = append(incPaths, fs...)
	}

	return incPaths
}

func generateProto(projectRoot, svrName string, pbFilePath string) error {
	var incPaths []string

	incPaths = getIncludePathList(pbFilePath)
	var outDir string
	var outPbPath string

	outDir = utils.AdjPathSep(projectRoot + "/src")

	// include proto
	target := fmt.Sprintf("--go_out=%s", outDir)

	if PD.GoPackageName == "" {
		outPbPath = fmt.Sprintf("%s%s%s.pb.go", outDir, sep, svrName)
	} else {
		n := PD.GoPackageName
		if runtime.GOOS == "windows" {
			n = strings.Replace(n, `/`, `\`, -1)
		}
		outPbPath = fmt.Sprintf("%s%s%s%s%s.pb.go", outDir, sep, n, sep, svrName)
	}

	var args []string
	for _, x := range incPaths {
		args = append(args, fmt.Sprintf("-I=%s", utils.AdjPathSep(x)))
	}
	args = append(args, target, pbFilePath)

	cmd := exec.Command("protoc", args...)

	log.Infof("protoc args %v", args)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		log.Infof("std out %s", cmd.Stdout)
		log.Errorf("std err %s", cmd.Stderr)
		log.Fatalf("exec protoc error %s", err)
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

	areas, gormMsgList, err := logic.InjectTagParseFile(outPbPath)
	if err != nil {
		log.Fatalf("parse error, err %s", err)
	}

	if len(areas) > 0 {
		err = logic.InjectTagWriteFile(outPbPath, areas)
		if err != nil {
			log.Fatalf("write fail, err %s", err)
		}
	}

	if len(gormMsgList) > 0 {
		err = logic.InjectTagWriteGormCode(outPbPath, gormMsgList)
		if err != nil {
			log.Fatalf("write fail, err %s", err)
		}
	}

	return nil
}

func findProjectRoot(mod string) string {
	abs, err := filepath.Abs(mod)
	if err != nil {
		log.Fatal("invalid path", mod)
	}

	for abs != "" {
		p := path.Join(abs, "src")
		res, _ := logic.IsDirectory(p)
		if res {
			return abs
		}

		prev := abs
		abs = filepath.Dir(abs)
		if prev == abs {
			break
		}
	}
	return ""
}

const (
	flagGenPb            = 1
	flagGenErrCode       = 1 << 1
	flagGenClient        = 1 << 2
	flagGenDef           = 1 << 3
	flagGenTypes         = 1 << 4
	flagRegisterOss      = 1 << 5
	flagRenTool          = 1 << 6
	flagSetStateDb       = 1 << 7
	flagSetStateRedis    = 1 << 8
	flagSetStateObjCache = 1 << 9
	flagGenDoc           = 1 << 10
	flagGenAll = 0xffffffff
)

func parsePbOrDie(protoFile string) *logic.ProtoDetect {
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

	pd := logic.NewProtoDetect()
	walkPb(definition, pd)

	return pd
}

func setMsgPtr() {
	total := len(pbList)
	for i := 0; i < total; i++ {
		pb := pbList[i]
		for _, v := range pb.Fields {
			var typ string
			if v.NormalField != nil {
				typ = v.NormalField.Type
			} else {
				typ = v.MapField.Type
			}

			// 1. find by mod + pb name
			dot := strings.LastIndex(typ, ".")

			var name string
			if dot > 0 {
				name = strings.Replace(typ, ".", "_", -1)
			} else {
				name = fmt.Sprintf("%s_%s", pb.ModName, typ)
			}
			if x, ok := logic.PbMap[name]; ok && x.NameDupCnt == 0 {
				v.Msg = x
				continue
			}

			// 1. find by parse order
			if dot > 0 {
				typ = typ[dot+1:]
			}

			bi := false
			switch typ {
			case "string", "uint32", "int32", "uint64", "int64", "bool", "bytes", "float", "double":
				bi = true
			}
			if bi {
				continue
			}

			ok := false
			for j := i; j >= 0; j-- {
				if typ == pbList[j].Name {
					v.Msg = pbList[j]
					ok = true
					break
				}
			}

			if !ok {
				for j := i + 1; j < total; j++ {
					if typ == pbList[j].Name {
						v.Msg = pbList[j]
						ok = true
						break
					}
				}
			}

			if !ok {
				log.Warnf("not found type %s", typ)
			}
		}
	}
}

func dumpMsg(pb *logic.PbMsg, level int) {
	if level == 0 {
		for i := 0; i < level; i++ {
			fmt.Printf("  ")
		}
		fmt.Printf("pb %s\n", pb.Name)
	}
	for _, x := range pb.Fields {
		for i := 0; i < level; i++ {
			fmt.Printf("  ")
		}

		if x.NormalField != nil {
			fmt.Printf(" field %s %s\n", x.NormalField.Type, x.NormalField.Name)
		} else {
			fmt.Printf(" field %s %s\n", x.MapField.Type, x.MapField.Name)
		}

		if x.Msg != nil {
			dumpMsg(x.Msg, level+1)
		}
	}
}

func genCode(flags int) {
	log.SetModName("rpc_gen")

	protoFile := tools_lib.OptStr("p")

	dbConf := tools_lib.OptStrDef("db", "")
	if dbConf != "" && strings.Index(dbConf, "$dispatch.mysql.") == -1 {
		dbConf = fmt.Sprintf("$dispatch.mysql.%s", dbConf)
	}

	redisConf := tools_lib.OptStrDef("redis", "")
	objCacheConf := tools_lib.OptStrDef("obj_cache", "")

	projectRoot := findProjectRoot(".")
	if projectRoot == "" {
		log.Fatalf("not found `src` path by search up of current directory")
	}

	incPaths := getIncludePathList(protoFile)

	logic.SetCurrentPb(protoFile)

	logic.PbIncPaths = incPaths

	PD = parsePbOrDie(protoFile)
	if PD.GoPackageName == "" {
		log.Fatalf("missed go_package option in %s", protoFile)
	}

	setMsgPtr()

	//for _, v := range pbList {
	//	dumpMsg(v, 0)
	//}

	log.Infof("project root %s", projectRoot)

	modPath := utils.AdjPathSep(
		fmt.Sprintf("%s/src/%s", projectRoot, PD.GoPackageName))
	logic.ModName = PD.GoPackageName
	if PD.SvrName == "" {
		PD.SvrName = PD.PackageName
	}

	err := os.MkdirAll(modPath, 0755)
	if err != nil {
		log.Fatalf("make dir fail, dir %s, err %s", modPath, err)
	}

	if (flags & flagGenPb) != 0 {
		err = generateProto(projectRoot, PD.SvrName, protoFile)
		if err != nil {
			log.Fatalf("Generate proto buffer file failed,error is %v", err)
		}
	}

	if (flags & flagGenTypes) != 0 {
		x := fmt.Sprintf("%s%sts", projectRoot, sep)
		err := os.MkdirAll(x, 0755)
		if err != nil {
			log.Fatalf("make dir fail, dir %s, err %s", x, err)
		}
		//err = logic.GenerateTypes(protoFile, x, incPaths)
		err = logic.GenerateTs(PD, protoFile, x)
		if err != nil {
			log.Fatalf("Generate typescript file failed,error is %v", err)
		}
	}

	if flags == flagRegisterOss {
		err = logic.RegisterOss(PD)
		if err != nil {
			log.Fatal(err)
		}
	}

	if flags == flagSetStateDb {
		err = logic.GenerateLogicStateDb(PD, modPath, dbConf, false)
		if err != nil {
			log.Fatalf("Generate logic state db file failed,error is %v", err)
		}
	}

	if flags == flagSetStateRedis {
		err = logic.GenerateLogicStateRedis(PD, modPath, redisConf, false)
		if err != nil {
			log.Fatalf("Generate logic state redis file failed,error is %v", err)
		}
	}

	if flags == flagSetStateObjCache {
		err = logic.GenerateLogicStateObjCache(PD, modPath, objCacheConf, false)
		if err != nil {
			log.Fatalf("Generate logic state redis file failed,error is %v", err)
		}
	}

	if flags == flagGenDoc {
		err = logic.GenerateDoc(PD)
		if err != nil {
			log.Fatalf("gen doc err %v", err)
		}
	}

	if (flags & flagGenErrCode) != 0 {
		err = logic.GenerateErrCode(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate errcode file failed,error is %v", err)
		}
	}

	if flags == flagGenAll && len(PD.RpcList) != 0 {
		err = logic.GenerateDef(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate Def file failed ,error is %v", err)
		}
		err = logic.GenerateClient(PD, modPath)
		if err != nil {
			log.Fatalf("Generate Client file failed,error is %v", err)
		}
		err = logic.GenerateLogic(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate logic file failed,error is %v", err)
		}
		err = logic.GenerateLogicCfg(PD, modPath)
		if err != nil {
			log.Fatalf("Generate logic cfg file failed,error is %v", err)
		}

		err = logic.GenerateServer(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate server file failed,error is %v", err)
		}

		err = logic.GenerateConf(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate conf file failed,error is %v", err)
		}
		err = logic.GenerateSupervisorConf(*PD, modPath)
		if err != nil {
			log.Fatalf("Generate supervisor conf file failed,error is %v", err)
		}

		err = logic.GenerateLogicStateDb(PD, modPath, dbConf, true)
		if err != nil {
			log.Fatalf("Generate logic state db file failed,error is %v", err)
		}
		err = logic.GenerateLogicStateRedis(PD, modPath, redisConf, true)
		if err != nil {
			log.Fatalf("Generate logic state redis file failed,error is %v", err)
		}
		err = logic.GenerateLogicStateObjCache(PD, modPath, objCacheConf, true)
		if err != nil {
			log.Fatalf("Generate logic state redis file failed,error is %v", err)
		}

		err = logic.GenerateTool(PD, modPath)
		if err != nil {
			log.Fatal("generate tool err %s", err)
		}
	}

	log.Infof("generate success, module path %s", modPath)

	//code := logic.AllErrCodes.GetAutoLoad("gialen", "ErrPasswordWrong")
	//log.Infof("code = %d", code)
}

func init() {
	log.SetModName("rpc_gen")

	//tools_lib.UsageTail = ``
}

// usage: -r <project root>
func NewProject() {
	root := tools_lib.OptStr("r")
	err := os.MkdirAll(root, 0777)
	if err != nil {
		log.Errorf("make dir `%s` fail %s", root, err)
		return
	}
	var subDirList = []string{
		"src",
		"proto",
		"ts",
	}
	for _, subDir := range subDirList {
		subPath := fmt.Sprintf("%s%s%s", root, sep, subDir)
		err := os.MkdirAll(subPath, 0777)
		if err != nil {
			log.Errorf("make dir `%s` fail %s", root, err)
			return
		}
	}
	log.Info("success")
}

// usage: -p <proto file> -I <proto include path sep by ,>
func GenAll() {
	genCode(flagGenAll)
}

// usage: -p <proto file> -I <proto include path sep by ,>
func Proto2Go() {
	genCode(flagGenPb)
}

// usage: -p <proto file> -I <proto include path sep by ,>
func Proto2ErrCode() {
	genCode(flagGenErrCode)
}

// usage: -p <proto file> -I <proto include path sep by ,>
func Proto2Types() {
	genCode(flagGenTypes)
}

// usage: -p <proto file> -I <proto include path sep by ,>
func RegisterOss() {
	genCode(flagRegisterOss)
}

// usage: -p <proto file> -I <proto include path sep by ,> -db <$dispatch.mysql.default>
func SetStateDb() {
	genCode(flagSetStateDb)
}

// usage: -p <proto file> -I <proto include path sep by ,> -redis <redis4session>
func SetStateRedis() {
	genCode(flagSetStateRedis)
}

// usage: -p <proto file> -I <proto include path sep by ,> -obj_cache <1>
func SetStateObjCache() {
	genCode(flagSetStateObjCache)
}

// usage: -s <server name> -a <address> -x <start or stop>
func ServerProfile() {
	server := tools_lib.OptStr("s")
	address := tools_lib.OptStrDef("a", "")
	action := tools_lib.OptStr("x")
	var isStart bool
	if action == "start" {
		isStart = true
	} else if action == "stop" {
		isStart = false
	} else {
		log.Infof("invalid action %s", action)
		return
	}

	err := rpc.ServerProfile(server, isStart, address)
	if err != nil {
		log.Error(err)
		return
	}
	log.Infof("success")
}

// usage: -p <proto file> -I <proto include path sep by ,>
func GenDoc() {
	genCode(flagGenDoc)
}

var pbRpcTmpl = `

	// @desc:
	// @error:
	rpc %s (%sReq) returns (%sRsp) {
		option(ext.CmdID) = %d;
	};`

// usage: -p <proto file> -r <rpc name> -l <list option, sep by ,>
func AddRpc() {
	p := tools_lib.OptStr("p")
	rpcName := tools_lib.OptStr("r")

	// validate name
	for _, v := range rpcName {
		if !((v >= 'a' && v <= 'z') || (v >= 'A' && v <= 'Z') ||
			(v >= '0' && v <= '9') || v == '_') {
			log.Errorf("invalid rpc name %s", rpcName)
			return
		}
	}

	if strings.HasSuffix(rpcName, "Req") || strings.HasSuffix(rpcName, "Rsp") {
		log.Errorf("rpc name can not has Req or Rsp suffix")
		return
	}

	listOption := tools_lib.OptStrDef("l", "")

	ctx, err := codegen.ParsePb(p)
	if err != nil {
		log.Errorf("parse pb err:%s", err)
		return
	}

	cmd := &codegen.Cmd{
		Name: rpcName,
		Args: map[string]string{
			"listOption": listOption,
		},
	}

	maxCmdId := ctx.GetMaxCmdId()

	r := ctx.GetRpc(rpcName)
	if r == nil {
		maxCmdId++
		x := fmt.Sprintf(
			pbRpcTmpl, rpcName, rpcName, rpcName,
			maxCmdId)

		err = ctx.InsertRpcBlock(x)
		if err != nil {
			log.Error(err)
			return
		}
	}

	if listOption != "" {
		enumName, enumBlock := cmd.GenListOptionEnum(rpcName)
		ctx.AppendEnumIfNotExisted(enumName, enumBlock)
	}

	ctx.AppendEmptyMsgIfNotExisted(rpcName + "Req")
	ctx.AppendEmptyMsgIfNotExisted(rpcName + "Rsp")

	ioutil.WriteFile(p, []byte(ctx.Content), 0666)

	log.Infof("gen success")
}
