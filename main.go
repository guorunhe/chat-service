package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

const (
	port = "1234"
)

type GroupInfo struct {
	sync.RWMutex
	Name     string
	UserList map[string]struct{}
}

var groupMap = make(map[string]GroupInfo)

func main() {
	listener, err := net.Listen("tcp", ":"+port)
	fmt.Println("Listening on port: ", port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		fmt.Println("Waiting for connection...")
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	fmt.Println("remote_addr: " + conn.RemoteAddr().String())
	for scanner.Scan() {
		handleText(scanner.Text(), conn)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func handleText(text string, conn net.Conn) {
	fmt.Println(text)
	command, params, err := commandHandler(text)
	if err != nil {
		fmt.Println(err)
		SendMessage(conn, "error: "+err.Error())
		return
	}
	if command == "m" {
		SendGroupMessage(conn, params[0], params[1])
		//SendMessage(conn, params[0])
		return
	}

	if command == "f" {
		//if len(params) < 2 {
		//	fmt.Println("command params is not enough")
		//	SendMessage(conn, "command params is not enough")
		//	return
		//}
		HandleCommand(conn, params[0], params[1:])
	}
}

func HandleCommand(conn net.Conn, command string, params []string) {
	switch command {
	case "login":
		userName := params[0]
		password := params[1]
		roleID, _ := strconv.ParseUint(params[2], 10, 8)
		fmt.Println("login: ", userName, password)
		loginOrRegister(userName, password, uint8(roleID), conn)
	case "create_group":
		CreateGroup(conn, params[0])
	case "group_list":
		GroupList(conn)
	case "join_group":
		JoinGroup(conn, params[0])
	case "leave_group":
		leaveGroup(conn, params[0])
	default:
		fmt.Println("command not found: ", command)
	}
}

var commandPrefixMap = map[string]struct{}{
	"f": {}, // 命令
	"m": {}, // 消息
}

// f {commandName} [params]
// f login userName password roleId
// f create_group group_name
// f group_list
// f join_group group_name
// f leave_group group_name
// m {group_name} {message}
func commandHandler(text string) (command string, params []string, err error) {
	sList := strings.Split(text, " ")
	if len(sList) <= 1 {
		err = errors.New(text + " is command format invalid")
		return
	}
	if _, ok := commandPrefixMap[sList[0]]; !ok {
		err = errors.New(text + " is not command")
		return
	}

	command = sList[0]
	params = sList[1:]
	return
}

type UesrInfo struct {
	sync.RWMutex
	UserName   string
	Password   string
	RemoteAddr string
	IsLogin    bool
	conn       *net.Conn
	RoleID     uint8
}

const (
	RoleIDAdmin = 1 // 管理员
	RoleIDUser  = 2 // 普通用户
)

var userInfoMap = make(map[string]*UesrInfo)
var remoteAddrToUserMap = make(map[string]string)

func loginOrRegister(userName, password string, roleID uint8, conn net.Conn) {
	// 检查用户是否存在
	userInfo, ok := userInfoMap[userName]
	if !ok {
		// 不存在则创建用户信息
		userInfo = &UesrInfo{UserName: userName, Password: password, conn: &conn, RoleID: roleID}
		userInfoMap[userName] = userInfo
	}
	// 检查用户是否登录
	userInfo.RLock()
	if userInfo.IsLogin {
		fmt.Println("user already login")
		userInfo.RUnlock()
		return
	}
	userInfo.RUnlock()
	// 用户未登录则登录
	userInfo.Lock()
	defer userInfo.Unlock()
	userInfo.IsLogin = true
	userInfo.RemoteAddr = conn.RemoteAddr().String()
	userInfo.conn = &conn
	remoteAddrToUserMap[conn.RemoteAddr().String()] = userName
	fmt.Println("login success")
	spew.Dump(userInfoMap, remoteAddrToUserMap)
}

func CreateGroup(conn net.Conn, groupName string) {
	userInfo, ok := userInfoMap[remoteAddrToUserMap[conn.RemoteAddr().String()]]
	if !ok {
		fmt.Println("user not login")
		SendMessage(conn, "user not login")
		return
	}
	if userInfo.RoleID != RoleIDAdmin {
		fmt.Println("user not admin")
		SendMessage(conn, "user not admin")
		return
	}

	if _, ok := groupMap[groupName]; ok {
		fmt.Println("group already exist")
		SendMessage(conn, "group already exist")
		return
	}

	groupMap[groupName] = GroupInfo{
		Name:     groupName,
		UserList: make(map[string]struct{}),
	}
	fmt.Println("create group success")
}

func JoinGroup(conn net.Conn, groupName string) {
	groupInfo, ok := groupMap[groupName]
	if !ok {
		fmt.Println("group not exist")
		SendMessage(conn, "group not exist")
		return
	}
	userName := remoteAddrToUserMap[conn.RemoteAddr().String()]
	groupInfo.Lock()
	defer groupInfo.Unlock()
	groupInfo.UserList[userName] = struct{}{}
}

func leaveGroup(conn net.Conn, groupName string) {
	groupInfo, ok := groupMap[groupName]
	if !ok {
		fmt.Println("group not exist")
		SendMessage(conn, "group not exist")
		return
	}
	userName := remoteAddrToUserMap[conn.RemoteAddr().String()]
	groupInfo.RLock()
	if _, ok := groupInfo.UserList[userName]; !ok {
		fmt.Println("user not in this group")
		SendMessage(conn, "user not in this group")
		groupInfo.RUnlock()
		return
	}
	groupInfo.RUnlock()
	groupInfo.Lock()
	delete(groupInfo.UserList, userName)
	groupInfo.Unlock()
}

func GroupList(conn net.Conn) {
	fmt.Println("group list")
	m := ""
	for _, groupInfo := range groupMap {
		fmt.Println(groupInfo)
		m += groupInfo.Name + "\t"
	}
	if m == "" {
		m = "no group"
	}
	m += "\n"
	SendMessage(conn, m)
}

func SendMessage(conn net.Conn, message string) {
	fmt.Println("send message: ", message)
	conn.Write([]byte(message + "\n"))
}

func SendGroupMessage(conn net.Conn, groupName, message string) {
	groupInfo, ok := groupMap[groupName]
	if !ok {
		fmt.Println("group not exist")
		SendMessage(conn, "group not exist")
		return
	}
	msg := formatMessage(conn, groupName, message)
	//groupInfo.UserList.Range(func(key, value interface{}) bool {
	//	userName := key.(string)
	//	userInfo, ok := userInfoMap[userName]
	//	if !ok {
	//		fmt.Println("user not exist")
	//		return false
	//	}
	//	SendMessage(*userInfo.conn, msg)
	//	return true
	//})
	curUserName := remoteAddrToUserMap[conn.RemoteAddr().String()]
	groupInfo.RLock()
	defer groupInfo.RUnlock()
	for userName, _ := range groupInfo.UserList {
		userInfo, ok := userInfoMap[userName]
		if !ok {
			fmt.Println("user not exist")
			continue
		}
		if userName == curUserName {
			continue
		}
		SendMessage(*userInfo.conn, msg)
	}
	fmt.Println("send group message: ", message)
}

func formatMessage(conn net.Conn, groupName, message string) string {
	return fmt.Sprintf("from: %s, to: %s, message: %s", remoteAddrToUserMap[conn.RemoteAddr().String()], groupName, message)
}
