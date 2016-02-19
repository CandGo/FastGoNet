// udp project main.go
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

//用户信息
type User struct {
	userName       string
	userAddr       *net.UDPAddr
	userListenConn *net.UDPConn
	chatToConn     *net.UDPConn
}

//数据包头，标识数据内容
var reflectString = map[string]string{
	"连接":   "connect  :",
	"在线":   "online   :",
	"聊天":   "chat     :",
	"在线用户": "get      :",
}

//服务器监听端口
const LISTENPORT = 16161
const CLIENTPORT = 16161

//缓冲区
const BUFFSIZE = 10240

var buff = make([]byte, BUFFSIZE)

//在线用户
var onlineUser = make([]User, 0)

//在线状态判断缓冲区
var onlineCheckAddr = make([]*net.UDPAddr, 0)

//错误处理
func HandleError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

//消息处理
func HandleMessage(udpListener *net.UDPConn) {
	n, addr, err := udpListener.ReadFromUDP(buff)
	HandleError(err)

	if n > 0 {
		msg := AnalyzeMessage(buff, n)

		switch msg[0] {
		//连接信息
		case "connect  ":
			//获取昵称+端口
			userName := msg[1]
			userListenPort := msg[2]
			//获取用户ip
			ip := AnalyzeMessage([]byte(addr.String()), len(addr.String()))
			//显示登录信息
			fmt.Println(" 昵称:", userName, " 地址:", ip[0], " 用户监听端口:", userListenPort, " 登录成功！")
			//创建对用户的连接，用于消息转发
			userAddr, err := net.ResolveUDPAddr("udp4", ip[0]+":"+userListenPort)
			HandleError(err)

			userConn, err := net.DialUDP("udp4", nil, userAddr)
			HandleError(err)

			//因为连接要持续使用，不能在这里关闭连接
			//defer userConn.Close()
			//添加到在线用户
			onlineUser = append(onlineUser, User{userName, addr, userConn, nil})

		case "online   ":
			//收到心跳包
			onlineCheckAddr = append(onlineCheckAddr, addr)

		case "outline  ":
			//退出消息，未实现
		case "chat     ":
			//会话请求
			//寻找请求对象
			index := -1
			for i := 0; i < len(onlineUser); i++ {
				if onlineUser[i].userName == msg[1] {
					index = i
				}
			}
			//将所请求对象的连接添加到请求者中
			if index != -1 {
				nowUser, _ := FindUser(addr)
				onlineUser[nowUser].chatToConn = onlineUser[index].userListenConn
			}
		case "get      ":
			//向请求者返回在线用户信息
			index, _ := FindUser(addr)
			onlineUser[index].userListenConn.Write([]byte("当前共有" + strconv.Itoa(len(onlineUser)) + "位用户在线"))
			for i, v := range onlineUser {
				onlineUser[index].userListenConn.Write([]byte("" + strconv.Itoa(i+1) + ":" + v.userName))
			}
		default:
			//消息转发
			//获取当前用户
			index, _ := FindUser(addr)
			//获取时间
			nowTime := time.Now()
			nowHour := strconv.Itoa(nowTime.Hour())
			nowMinute := strconv.Itoa(nowTime.Minute())
			nowSecond := strconv.Itoa(nowTime.Second())
			//请求会话对象是否存在
			if onlineUser[index].chatToConn == nil {
				onlineUser[index].userListenConn.Write([]byte("对方不在线"))
			} else {
				onlineUser[index].chatToConn.Write([]byte(onlineUser[index].userName + " " + nowHour + ":" + nowMinute + ":" + nowSecond + "\n" + msg[0]))
			}

		}
	}
}

//消息解析，[]byte -> []string
func AnalyzeMessage(buff []byte, len int) []string {
	analMsg := make([]string, 0)
	strNow := ""
	for i := 0; i < len; i++ {
		if string(buff[i:i+1]) == ":" {
			analMsg = append(analMsg, strNow)
			strNow = ""
		} else {
			strNow += string(buff[i : i+1])
		}
	}
	analMsg = append(analMsg, strNow)
	return analMsg
}

//寻找用户，返回（位置，是否存在）
func FindUser(addr *net.UDPAddr) (int, bool) {
	alreadyhave := false
	index := -1
	for i := 0; i < len(onlineUser); i++ {

		if onlineUser[i].userAddr.String() == addr.String() {
			alreadyhave = true
			index = i
			break
		}
	}
	return index, alreadyhave
}

//处理用户在线信息（暂时仅作删除用户使用）
func HandleOnlineMessage(addr *net.UDPAddr, state bool) {
	index, alreadyhave := FindUser(addr)
	if state == false {
		if alreadyhave {
			onlineUser = append(onlineUser[:index], onlineUser[index+1:len(onlineUser)]...)
		}
	}
}

//在线判断，心跳包处理，每5s查看一次所有已在线用户状态
func OnlineCheck() {
	for {
		onlineCheckAddr = make([]*net.UDPAddr, 0)
		sleepTimer := time.NewTimer(time.Second * 5)
		<-sleepTimer.C
		for i := 0; i < len(onlineUser); i++ {
			haved := false
		FORIN:
			for j := 0; j < len(onlineCheckAddr); j++ {
				if onlineUser[i].userAddr.String() == onlineCheckAddr[j].String() {
					haved = true
					break FORIN
				}
			}
			if !haved {
				fmt.Println(onlineUser[i].userAddr.String() + "退出！")
				HandleOnlineMessage(onlineUser[i].userAddr, false)
				i--
			}

		}
	}
}

func main() {
	//监听地址
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(LISTENPORT))
	HandleError(err)
	//监听连接
	udpListener, err := net.ListenUDP("udp4", udpAddr)
	HandleError(err)

	defer udpListener.Close()

	fmt.Println("开始监听：")

	//在线状态判断
	go OnlineCheck()

	for {
		//消息处理
		HandleMessage(udpListener)
	}

}

////错误消息处理
//func HandleError(err error) {
//	if err != nil {
//		fmt.Println(err.Error())
//		os.Exit(2)
//	}
//}

//发送消息
func SendMessage(udpConn *net.UDPConn) {
	scaner := bufio.NewScanner(os.Stdin)

	for scaner.Scan() {
		if scaner.Text() == "exit" {
			return
		}
		udpConn.Write([]byte(scaner.Text()))
	}
}

////接收消息
//func HandleMessage(udpListener *net.UDPConn) {
//	for {
//		n, _, err := udpListener.ReadFromUDP(buff)
//		HandleError(err)

//		if n > 0 {
//			fmt.Println(string(buff[:n]))
//		}
//	}
//}

/*
func AnalyzeMessage(buff []byte, len int) ([]string) {
    analMsg := make([]string, 0)
    strNow := ""
    for i := 0; i < len; i++ {
        if string(buff[i:i + 1]) == ":" {
            analMsg = append(analMsg, strNow)
            strNow = ""
        } else {
            strNow += string(buff[i:i + 1])
        }
    }
    analMsg = append(analMsg, strNow)
    return analMsg
}*/
//发送心跳包
func SendOnlineMessage(udpConn *net.UDPConn) {
	for {
		//每间隔1s向服务器发送一次在线信息
		udpConn.Write([]byte(reflectString["在线"]))
		sleepTimer := time.NewTimer(time.Second)
		<-sleepTimer.C
	}
}

func ClientMain() {
	//判断命令行参数，参数应该为服务器ip
	if len(os.Args) != 2 {
		fmt.Println("程序命令行参数错误！")
		os.Exit(2)
	}
	//获取ip
	host := os.Args[1]

	//udp地址
	udpAddr, err := net.ResolveUDPAddr("udp4", host+":"+strconv.Itoa(CLIENTPORT))
	HandleError(err)

	//udp连接
	udpConn, err := net.DialUDP("udp4", nil, udpAddr)
	HandleError(err)

	//本地监听端口
	newSeed := rand.NewSource(int64(time.Now().Second()))
	newRand := rand.New(newSeed)
	randPort := newRand.Intn(30000) + 10000

	//本地监听udp地址
	udpLocalAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+strconv.Itoa(randPort))
	HandleError(err)

	//本地监听udp连接
	udpListener, err := net.ListenUDP("udp4", udpLocalAddr)
	HandleError(err)

	//fmt.Println("监听", randPort, "端口")

	//用户昵称
	userName := ""
	fmt.Printf("请输入昵称：")
	fmt.Scanf("%s", &userName)

	//向服务器发送连接信息（昵称+本地监听端口）
	udpConn.Write([]byte(reflectString["连接"] + userName + ":" + strconv.Itoa(randPort)))

	//关闭端口
	defer udpConn.Close()
	defer udpListener.Close()

	//发送心跳包
	go SendOnlineMessage(udpConn)
	//接收消息
	go HandleMessage(udpListener)

	command := ""

	for {
		//获取命令
		fmt.Scanf("%s", &command)
		switch command {
		case "chat":
			people := ""
			//fmt.Printf("请输入对方昵称：")
			fmt.Scanf("%s", &people)
			//向服务器发送聊天对象信息
			udpConn.Write([]byte(reflectString["聊天"] + people))
			//进入会话
			SendMessage(udpConn)
			//退出会话
			fmt.Println("退出与" + people + "的会话")
		case "get":
			//请求在线情况信息
			udpConn.Write([]byte(reflectString["在线用户"]))
		}
	}
}
