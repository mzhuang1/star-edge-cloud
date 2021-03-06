package service

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"star-edge-cloud/edge/core/config"
	"star-edge-cloud/edge/core/device"
	"star-edge-cloud/edge/core/extension"
	"star-edge-cloud/edge/core/share"
	"star-edge-cloud/edge/models"
	"star-edge-cloud/edge/utils/common"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// CoreServer - 元数据服务
type CoreServer struct {
	DevManager         device.DeviceManager
	ExtManager         extension.ExtentionManager
	StoreManager       extension.StoreManager
	LogManager         extension.LogManager
	SchedulerManager   extension.SchedulerManager
	RulesEngineManager extension.RulesEngineManager
	Conf               *config.Config
	exts               []models.Extension
}

// Start - 启动服务
func (cs *CoreServer) Start() error {
	if len(cs.exts) == 0 {
		cs.exts = append(cs.exts, models.Extension{Name: "存储服务", Type: "store", FileName: "store"})
		cs.exts = append(cs.exts, models.Extension{Name: "日志服务", Type: "log", FileName: "log"})
		cs.exts = append(cs.exts, models.Extension{Name: "调度服务", Type: "scheduler", FileName: "scheduler"})
		cs.exts = append(cs.exts, models.Extension{Name: "规则引擎", Type: "rule", FileName: "rules_engine"})
	}
	// 不要自动启动
	// if err := cs.StoreManager.Run(); err == nil {
	// 	cs.exts[0].Status = 2
	// }
	// if err := cs.LogManager.Run(); err == nil {
	// 	cs.exts[1].Status = 2
	// }
	// if err := cs.SchedulerManager.Run(); err == nil {
	// 	cs.exts[2].Status = 2
	// }
	// if err := cs.RulesEngineManager.Run(); err == nil {
	// 	cs.exts[3].Status = 2
	// }

	mux := cs.makeMuxRouter()
	// cs.Loghelper.Write(&models.LogInfo{ID: common.GetGUID(), Message: "Core服务启动，开始监听端口 :" + cs.Conf.Port, Level: 1, Time: time.Now()})
	log.Println("服务启动，开始监听端口 :", cs.Conf.Port)
	s := &http.Server{
		Addr:           ":" + cs.Conf.Port,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

// create handlers
func (cs *CoreServer) makeMuxRouter() http.Handler {
	muxRouter := mux.NewRouter()
	muxRouter.HandleFunc("/{category:html|js|css|images}/{name:.*}", cs.handleStaticResource)
	muxRouter.HandleFunc("/api/device/add", cs.handleAddDevice).Methods("POST")
	muxRouter.HandleFunc("/api/device/remove", cs.handleRemoveDevice).Methods("POST")
	muxRouter.HandleFunc("/api/device/operate", cs.handleOperateDevice).Methods("POST")
	muxRouter.HandleFunc("/api/device/all", cs.handleAllDevice).Methods("POST")
	muxRouter.HandleFunc("/api/device/count", cs.handleCountDevice).Methods("POST")
	// muxRouter.HandleFunc("/api/device/stop", cs.handleStopDevice).Methods("POST")
	muxRouter.HandleFunc("/api/device/status", cs.handleGetDeviceStatus).Methods("POST")
	muxRouter.HandleFunc("/api/extension/add", cs.handleAddExtension).Methods("POST")
	muxRouter.HandleFunc("/api/extension/remove", cs.handleRemoveExtension).Methods("POST")
	muxRouter.HandleFunc("/api/extension/operate", cs.handleOperateExtension).Methods("POST")
	// muxRouter.HandleFunc("/api/extension/stop", cs.handleStopExtension).Methods("POST")
	muxRouter.HandleFunc("/api/extension/all", cs.handleAllExtension).Methods("POST")
	muxRouter.HandleFunc("/api/extension/count", cs.handleCountExtension).Methods("POST")
	muxRouter.HandleFunc("/api/system/all", cs.handleAllSystemExtension).Methods("POST")
	muxRouter.HandleFunc("/api/system/operate", cs.handleOperateSystemExtension).Methods("POST")
	muxRouter.HandleFunc("/api/store/run", cs.handleRunStore).Methods("POST")
	muxRouter.HandleFunc("/api/store/stop", cs.handleStopStore).Methods("POST")
	muxRouter.HandleFunc("/api/store/status", cs.handleStoreStatus).Methods("POST")
	muxRouter.HandleFunc("/api/log/run", cs.handleRunLog).Methods("POST")
	muxRouter.HandleFunc("/api/log/stop", cs.handleStopLog).Methods("POST")
	muxRouter.HandleFunc("/api/log/status", cs.handleLogStatus).Methods("POST")
	muxRouter.HandleFunc("/api/rulesengine/run", cs.handleRunRulesEngine).Methods("POST")
	muxRouter.HandleFunc("/api/rulesengine/stop", cs.handleStopRulesEngine).Methods("POST")
	muxRouter.HandleFunc("/api/rulesengine/status", cs.handleRulesEngineStatus).Methods("POST")
	muxRouter.HandleFunc("/api/rulesengine/edit", cs.handleEditRules).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/run", cs.handleRunScheduler).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/stop", cs.handleStopScheduler).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/status", cs.handleSchedulerStatus).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/add", cs.handleAddSchedulerTask).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/remove", cs.handleRemoveSchedulerTask).Methods("POST")
	muxRouter.HandleFunc("/api/scheduler/list", cs.handleListSchedulerTask).Methods("POST")
	muxRouter.HandleFunc("/api/help/content", cs.handleHelp).Methods("Get")
	// muxRouter.HandleFunc("/api/{name:.*}", cs.handle)
	return muxRouter
}

func (cs *CoreServer) handleStaticResource(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	name := params["name"]
	path := fmt.Sprintf("./website/%[1]s", name)
	content := common.ReadString(path)
	io.WriteString(w, content)
}

func (cs *CoreServer) handleAddDevice(w http.ResponseWriter, r *http.Request) {
	// 创建设备目录和设备运行文件
	file, head, err := r.FormFile("file")
	if err != nil {
		io.WriteString(w, "未上传文件")
	}
	defer file.Close()

	// 读取设备信息
	name := r.Form.Get("dev_name")
	conf := r.Form.Get("conf")
	listeners := r.Form.Get("listeners")
	cmdaddr := r.Form.Get("cmd_addr")
	logurl := r.Form.Get("logurl")
	now := time.Now().Format("2006-01-02 15:04:05")
	device := &models.Device{
		ID:            common.GetGUID(),
		Name:          name,
		Conf:          conf,
		FileName:      head.Filename,
		ServerAddress: cmdaddr,
		Listeners:     listeners,
		RegistryTime:  now,
		LogBaseURL:    logurl}
	if err := cs.DevManager.AddDevice(file, device); err == nil {
		io.WriteString(w, "success.")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleRemoveDevice(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	if err := cs.DevManager.RemoveDevice(id); err == nil {
		io.WriteString(w, "success.")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleOperateDevice(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	handle := r.Form.Get("handle")
	if handle == "run" {
		err = cs.DevManager.Run(id)
		if err == nil {
			io.WriteString(w, "running")
		} else {
			io.WriteString(w, err.Error())
		}
	} else {
		err = cs.DevManager.Stop(id)
		if err == nil {
			io.WriteString(w, "stopped")
		} else {
			io.WriteString(w, err.Error())
		}
	}
}

func (cs *CoreServer) handleAllDevice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	devices, _ := cs.DevManager.QueryAllDevice()
	// 更新状态
	for index, item := range devices {
		if status := cs.DevManager.GetStatus(&item); status != item.Status {
			devices[index].Status = status
			cs.DevManager.UpdateDevice(&devices[index])
		}
	}
	data, _ := json.Marshal(devices)
	io.WriteString(w, string(data[:]))
}

func (cs *CoreServer) handleCountDevice(w http.ResponseWriter, r *http.Request) {
	// id := r.Form.Get("id")
	// w.Header().Set("Content-Type", "text/plain")
	devices, _ := cs.DevManager.QueryAllDevice()
	var runNum, totalNum int
	for _, d := range devices {
		if d.Status == 2 {
			runNum++
		}
		totalNum++
	}
	json := fmt.Sprintf(`{"Running":%[1]d,"Total":%[2]d}`, runNum, totalNum)
	// json := fmt.Sprintf(`{"Running":%[1]d,"Total":%[2]d}`, 6, 5)
	io.WriteString(w, json)
}

func (cs *CoreServer) handleGetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	dev, _ := cs.DevManager.GetDevice(id)
	status := cs.DevManager.GetStatus(dev)
	io.WriteString(w, strconv.Itoa(status))
}

func (cs *CoreServer) handleAddExtension(w http.ResponseWriter, r *http.Request) {
	// 创建设备目录和设备运行文件
	file, head, err := r.FormFile("file")
	if err == nil {
		io.WriteString(w, "未上传文件")
	}
	defer file.Close()

	// 读取服务信息
	name := r.Form.Get("ext_name")
	conf := r.Form.Get("conf")
	listeners := r.Form.Get("listeners")
	cmdaddr := r.Form.Get("cmd_addr")
	logurl := r.Form.Get("logurl")
	now := time.Now().Format("2006-01-02 15:04:05")

	ext := &models.Extension{
		ID:            common.GetGUID(),
		Name:          name,
		Conf:          conf,
		FileName:      head.Filename,
		ServerAddress: cmdaddr,
		Listeners:     listeners,
		RegistryTime:  now,
		Type:          "algorithm",
		LogBaseURL:    logurl}

	if err := cs.ExtManager.AddExtension(file, ext); err == nil {
		io.WriteString(w, "success.")
	} else {
		io.WriteString(w, err.Error())
	}

}

func (cs *CoreServer) handleRemoveExtension(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	if err := cs.ExtManager.RemoveExtension(id); err == nil {
		io.WriteString(w, "success.")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleOperateExtension(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	handle := r.Form.Get("handle")
	if handle == "run" {
		err = cs.ExtManager.Run(id)
		if err == nil {
			io.WriteString(w, "running")
		} else {
			io.WriteString(w, err.Error())
		}
	} else {
		err = cs.ExtManager.Stop(id)
		if err == nil {
			io.WriteString(w, "stopped")
		} else {
			io.WriteString(w, err.Error())
		}
	}
}

func (cs *CoreServer) handleCountExtension(w http.ResponseWriter, r *http.Request) {
	exts, _ := cs.ExtManager.QueryAllExtension("algorithm")
	var runNum, totalNum int
	for _, e := range exts {
		if e.Status == 2 {
			runNum++
		}
		totalNum++
	}
	json := fmt.Sprintf(`{"Running":%[1]d,"Total":%[2]d}`, runNum, totalNum)
	// json := fmt.Sprintf(`{"Running":%[1]d,"Total":%[2]d}`, 6, 5)
	io.WriteString(w, json)
}

func (cs *CoreServer) handleAllExtension(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	// err := r.ParseForm()
	// if err != nil {
	// 	log.Println("解析表单数据失败!")
	// }
	// etype := r.Form.Get("type")
	exts, _ := cs.ExtManager.QueryAllExtension("algorithm")
	// 更新状态
	for index, item := range exts {
		if status := cs.ExtManager.GetStatus(&exts[index]); status != item.Status {
			exts[index].Status = status
			cs.ExtManager.UpdateExtension(&exts[index])
		}
	}
	data, _ := json.Marshal(exts)
	io.WriteString(w, string(data[:]))

}

func (cs *CoreServer) handleAllSystemExtension(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	// 更新状态
	for index, item := range cs.exts {
		if status := common.ExecCheckStatus(share.WorkingDir, "./", item.FileName, "status"); common.StatusCovert(status) != item.Status {
			cs.exts[index].Status = common.StatusCovert(status)
			// cs.ExtManager.UpdateExtension(&cs.exts[index])
		}
	}
	data, _ := json.Marshal(cs.exts)
	io.WriteString(w, string(data[:]))

}

func (cs *CoreServer) handleOperateSystemExtension(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	_type := r.Form.Get("type")
	handle := r.Form.Get("handle")
	if handle == "run" {
		switch _type {
		case "store":
			err = cs.StoreManager.Run()
			cs.exts[0].Status = 2
		case "log":
			err = cs.LogManager.Run()
			cs.exts[1].Status = 2
		case "scheduler":
			err = cs.SchedulerManager.Run()
			cs.exts[2].Status = 2
		case "rule":
			err = cs.RulesEngineManager.Run()
			cs.exts[3].Status = 2
		}

	} else {
		switch _type {
		case "store":
			err = cs.StoreManager.Stop()
			cs.exts[0].Status = 1
		case "log":
			err = cs.LogManager.Stop()
			cs.exts[1].Status = 1
		case "scheduler":
			err = cs.SchedulerManager.Stop()
			cs.exts[2].Status = 1
		case "rule":
			err = cs.RulesEngineManager.Stop()
			cs.exts[3].Status = 1
		}
	}
	data, _ := json.Marshal(cs.exts)
	io.WriteString(w, string(data[:]))
}

func (cs *CoreServer) handleStopExtension(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	id := r.Form.Get("id")
	err = cs.ExtManager.Stop(id)
	if err == nil {
		io.WriteString(w, "stopped")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleRunStore(w http.ResponseWriter, r *http.Request) {
	err := cs.StoreManager.Run()
	if err == nil {
		io.WriteString(w, "running")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleStopStore(w http.ResponseWriter, r *http.Request) {
	err := cs.StoreManager.Stop()
	if err == nil {
		io.WriteString(w, "stopped")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleStoreStatus(w http.ResponseWriter, r *http.Request) {
	status := cs.StoreManager.GetStatus()
	switch status {
	case 1:
		io.WriteString(w, "stopped")
	case 2:
		io.WriteString(w, "running")
	default:
		io.WriteString(w, "error")
	}
}

func (cs *CoreServer) handleRunLog(w http.ResponseWriter, r *http.Request) {
	err := cs.LogManager.Run()
	if err == nil {
		io.WriteString(w, "running")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleStopLog(w http.ResponseWriter, r *http.Request) {
	err := cs.LogManager.Stop()
	if err == nil {
		io.WriteString(w, "stopped")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleLogStatus(w http.ResponseWriter, r *http.Request) {
	status := cs.LogManager.GetStatus()
	switch status {
	case 1:
		io.WriteString(w, "stopped")
	case 2:
		io.WriteString(w, "running")
	default:
		io.WriteString(w, "error")
	}
}

func (cs *CoreServer) handleRunRulesEngine(w http.ResponseWriter, r *http.Request) {
	err := cs.RulesEngineManager.Run()
	if err == nil {
		io.WriteString(w, "running")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleStopRulesEngine(w http.ResponseWriter, r *http.Request) {
	err := cs.RulesEngineManager.Stop()
	if err == nil {
		io.WriteString(w, "stopped")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleRulesEngineStatus(w http.ResponseWriter, r *http.Request) {
	status := cs.RulesEngineManager.GetStatus()
	switch status {
	case 1:
		io.WriteString(w, "stopped")
	case 2:
		io.WriteString(w, "running")
	default:
		io.WriteString(w, "error")
	}
}

func (cs *CoreServer) handleEditRules(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	// name := r.Form.Get("rules_name")
	conf := r.Form.Get("conf")
	rules := &models.Rules{}
	if err = xml.Unmarshal([]byte(conf), rules); err != nil {
		io.WriteString(w, err.Error())
		return
	}

	// client := &thttp.RestClient{}
	// if _, err = client.PostRules(cs.Conf.RulesAddr+"rules", rules); err != nil {
	// 	io.WriteString(w, err.Error())
	// } else {
	// 	io.WriteString(w, "success")
	// }

}

func (cs *CoreServer) handleRunScheduler(w http.ResponseWriter, r *http.Request) {
	err := cs.SchedulerManager.Run()
	if err == nil {
		io.WriteString(w, "running")
	} else {
		io.WriteString(w, err.Error())
	}

}

func (cs *CoreServer) handleStopScheduler(w http.ResponseWriter, r *http.Request) {
	err := cs.SchedulerManager.Stop()
	if err == nil {
		io.WriteString(w, "stopped")
	} else {
		io.WriteString(w, err.Error())
	}
}

func (cs *CoreServer) handleSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	status := cs.SchedulerManager.GetStatus()
	switch status {
	case 1:
		io.WriteString(w, "stopped")
	case 2:
		io.WriteString(w, "running")
	default:
		io.WriteString(w, "error")
	}
}

func (cs *CoreServer) handleAddSchedulerTask(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}

	// name := r.Form.Get("task_name")
	// addr := r.Form.Get("task_addr")
	// task := &models.SchedulerTask{Name: name, Address: addr}
	// client := &thttp.RestClient{}
	// if _, err = client.PostSchedulerTask(cs.Conf.SchedulerTaskAddr+"scheduler", task); err != nil {
	// 	io.WriteString(w, err.Error())
	// } else {
	// 	io.WriteString(w, "success")
	// }
}

func (cs *CoreServer) handleRemoveSchedulerTask(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	// id := r.Form.Get("id")
	// client := &thttp.RestClient{}
	// if _, err = client.PostCommand(cs.Conf.SchedulerTaskAddr+"command",
	// 	&models.Command{Type: "remove", Data: []byte(id)}); err != nil {
	// 	io.WriteString(w, err.Error())
	// } else {
	// 	io.WriteString(w, "success")
	// }
}

func (cs *CoreServer) handleListSchedulerTask(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("解析表单数据失败!")
	}
	// client := &thttp.RestClient{}
	// if data, err := client.PostCommand(cs.Conf.SchedulerTaskAddr+"command",
	// 	&models.Command{Type: "list"}); err != nil {
	// 	io.WriteString(w, err.Error())
	// } else {
	// 	arr := []models.SchedulerTask{}
	// 	json.Unmarshal(data.Message, &arr)
	// 	io.WriteString(w, "success")
	// }
}

func (cs *CoreServer) handleHelp(w http.ResponseWriter, r *http.Request) {
	f, err := os.Open("./doc/help.md")
	if err != nil {
		io.WriteString(w, "fail")
	}
	defer f.Close()
	data, _ := ioutil.ReadAll(f)
	io.WriteString(w, string(data[:]))
}
