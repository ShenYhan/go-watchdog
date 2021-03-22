/**
  @author: yhan
  @date: 2021/3/22
  @note:
**/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gogf/gf/net/ghttp"
	"github.com/lxn/walk"
	"github.com/rodolfoag/gow32"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

type MyWindow struct {
	*walk.MainWindow
	ni *walk.NotifyIcon
}

var (
	logFileName = flag.String("log", "LogInfo.log", "Log file name")
)

const (
	ERRORCOUNT = 5
)

var interval int
var config Server
var gclient *ghttp.Client
var ticker *time.Ticker
var errCount int

func main() {
	_, err := gow32.CreateMutex("SomeMutexName")
	if err == nil {
		initSystem()
		go CheckHttpConn()
		mw := NewMyWindow()
		mw.AddNotifyIcon()
		mw.Run()
	} else {
		walk.MsgBox(nil, "提示", "应用程序已运行!", walk.MsgBoxIconInformation)
	}
}

func initSystem() {
	initClient()
	initLog()
	config = readConfig()
}

func initClient() {
	gclient = ghttp.NewClient()
	gclient.SetHeader("Content-Type", "application/json")
	gclient.SetTimeout(3 * time.Second)
}

func initLog() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	//set logfile Stdout
	logFile, logErr := os.OpenFile(*logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if logErr != nil {
		fmt.Println("Fail to find", *logFile, "LogInfo start Failed")
		os.Exit(1)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func NewMyWindow() *MyWindow {
	mw := new(MyWindow)
	var err error
	mw.MainWindow, err = walk.NewMainWindow()
	checkError(err)
	return mw
}

func (mw *MyWindow) AddNotifyIcon() {
	var err error
	mw.ni, err = walk.NewNotifyIcon(mw.MainWindow)
	checkError(err)
	mw.ni.SetVisible(true)

	icon, err := walk.Resources.Icon("./rc.ico")
	checkError(err)
	mw.SetIcon(icon)
	mw.ni.SetIcon(icon)

	startAction := mw.addAction(nil, "开始")
	stopAction := mw.addAction(nil, "停止")
	stopAction.SetEnabled(false)

	//开始
	startAction.Triggered().Attach(func() {
		interval = config.IntervalMS
		startAction.SetEnabled(false)
		stopAction.SetEnabled(true)
	})
	//停止
	stopAction.Triggered().Attach(func() {
		interval = 0
		stopAction.SetEnabled(false)
		startAction.SetEnabled(true)
	})
	//退出
	mw.addAction(nil, "退出").Triggered().Attach(func() {
		mw.ni.Dispose()
		mw.Dispose()
		walk.App().Exit(0)
		os.Exit(1)
	})
}

func (mw *MyWindow) addMenu(name string) *walk.Menu {
	helpMenu, err := walk.NewMenu()
	checkError(err)
	help, err := mw.ni.ContextMenu().Actions().AddMenu(helpMenu)
	checkError(err)
	help.SetText(name)

	return helpMenu
}

func (mw *MyWindow) addAction(menu *walk.Menu, name string) *walk.Action {
	action := walk.NewAction()
	action.SetText(name)
	if menu != nil {
		menu.Actions().Add(action)
	} else {
		mw.ni.ContextMenu().Actions().Add(action)
	}

	return action
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	ProgramPath string
	ProgramName string
	Arg         string
	Target      string
	IntervalMS  int
}

func readConfig() Server {
	JsonParse := NewJsonStruct()
	v := Server{}
	JsonParse.Load("./Config.json", &v) //相对路径,config.json文件和main.go文件处于一同目录下
	interval = v.IntervalMS
	return v
}

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) {
	//ReadFile函数会读取文件的全部内容，并将结果以[]byte类型返回
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	//读取的数据为json格式，需要进行解码
	err = json.Unmarshal(data, v)
	if err != nil {
		return
	}
}

func CheckHttpConn() {
	ticker = time.NewTicker(time.Second)
	for _ = range ticker.C {
		webStr := config.Target
		if webStr != "" {
			resp, err := gclient.Get(webStr)
			if err != nil {
				errCount += 1
				log.Println(fmt.Sprintf("fail to get response error %v\n", err))
			} else {
				defer resp.Close()
				if resp.StatusCode != 200 {
					errCount += 1
				} else {
					errCount = 0
				}
			}
		}
		if errCount == ERRORCOUNT {
			restartServer(config)
			errCount = 0
		}
	}
}

func restartServer(cfg Server) {
	log.Println("prepare to restart...")
	cmd := exec.Command(cfg.ProgramPath+"/"+cfg.ProgramName, cfg.Arg)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	// 设置接收
	var out bytes.Buffer
	cmd.Stdout = &out

	// 执行
	err := cmd.Run()

	if err != nil {
		log.Fatalf("fail to restart server error %v\n", err)
	} else {
		log.Println("success restart server")
	}
}
