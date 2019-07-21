package main

import (
	"brick/tools/tools_builder/tools_lib"
)

func wrapperNewProject() {
	NewProject()
}

func wrapperGenAll() {
	GenAll()
}

func wrapperProto2Go() {
	Proto2Go()
}

func wrapperProto2ErrCode() {
	Proto2ErrCode()
}

func wrapperProto2Types() {
	Proto2Types()
}

func wrapperRegisterOss() {
	RegisterOss()
}

func wrapperSetStateDb() {
	SetStateDb()
}

func wrapperSetStateRedis() {
	SetStateRedis()
}

func wrapperSetStateObjCache() {
	SetStateObjCache()
}

func wrapperServerProfile() {
	ServerProfile()
}

func wrapperGenDoc() {
	GenDoc()
}

func wrapperAddRpc() {
	AddRpc()
}

func main() {
	tools_lib.Register("NewProject", `-r <project root>`, wrapperNewProject)
	tools_lib.Register("GenAll", `-p <proto file> -I <proto include path sep by ,>`, wrapperGenAll)
	tools_lib.Register("Proto2Go", `-p <proto file> -I <proto include path sep by ,>`, wrapperProto2Go)
	tools_lib.Register("Proto2ErrCode", `-p <proto file> -I <proto include path sep by ,>`, wrapperProto2ErrCode)
	tools_lib.Register("Proto2Types", `-p <proto file> -I <proto include path sep by ,>`, wrapperProto2Types)
	tools_lib.Register("RegisterOss", `-p <proto file> -I <proto include path sep by ,>`, wrapperRegisterOss)
	tools_lib.Register("SetStateDb", `-p <proto file> -I <proto include path sep by ,> -db <$dispatch.mysql.default>`, wrapperSetStateDb)
	tools_lib.Register("SetStateRedis", `-p <proto file> -I <proto include path sep by ,> -redis <redis4session>`, wrapperSetStateRedis)
	tools_lib.Register("SetStateObjCache", `-p <proto file> -I <proto include path sep by ,> -obj_cache <1>`, wrapperSetStateObjCache)
	tools_lib.Register("ServerProfile", `-s <server name> -a <address> -x <start or stop>`, wrapperServerProfile)
	tools_lib.Register("GenDoc", `-p <proto file> -I <proto include path sep by ,>`, wrapperGenDoc)
	tools_lib.Register("AddRpc", `-p <proto file> -r <rpc name> -l <list option, sep by ,>`, wrapperAddRpc)
	tools_lib.Run()
}
