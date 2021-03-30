package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
	
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	
	"JDSeckill/conf"
	"JDSeckill/jd_seckill"
)

var (
	wg        *sync.WaitGroup
)

func jdCMD() *cobra.Command {
	cmd := cobra.Command{
		Use:   "run",
		Short: "京东秒杀服务",
		Long:  "京东秒杀服务",
		Run: func(cmd *cobra.Command, args []string) {
			logger := zap.L().With(zap.String("jd", "[seckill]"))
			client := resty.New()
			run(client, logger)
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			conf.ParseConfig(cfgPath)

		},
	}
	
	cmd.Flags().StringVar(&cfgPath, "config", "", "config file")
	
	return &cmd
}

func init() {
	ROOTCMD.AddCommand(jdCMD())
}

func run(client *resty.Client, logger *zap.Logger) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 用户登录
	user := jd_seckill.NewUser(client, conf.C, logger)
	wlfstkSmdl, err := user.QrLogin()
	if err != nil {
		logger.Error("用户扫码失败", zap.Error(err))
		os.Exit(0)
	}
	ticket, err := user.QrcodeTicket(wlfstkSmdl)
	if err != nil{
		logger.Error("获取令牌失败", zap.Error(err))
		os.Exit(0)
	}

	_, err = user.TicketInfo(ticket)
	if err != nil {
		logger.Error("登陆失败", zap.Error(err))
		return
	}
	
	// 刷新用户状态和获取用户信息
	logger.Info("登录成功")
	if status := user.RefreshStatus(); status == nil {
		seckill := preper(logger, *user, client)
		// 开启任务
		logger.Info("时间到达，开始执行……")
		start(seckill, 5)
		wg.Wait()
	}
}

func getJdTime(logger *zap.Logger) (int64, error) {
	req := resty.New()
	resp, err := req.NewRequest().Get("https://api.m.jd.com/client.action?functionId=queryMaterialProducts&client=wh5")
	if err != nil || resp.StatusCode() != http.StatusOK {
		logger.Error("获取京东服务器时间失败", zap.Error(err))
		return 0, errors.New("获取京东服务器时间失败")
	}
	return gjson.Get(string(resp.Body()), "currentTime2").Int(), nil
}

func start(seckill *jd_seckill.Seckill, taskNum int) {
	for i := 1; i <= taskNum; i++ {
		go func(seckill *jd_seckill.Seckill) {
			seckill.RequestSeckillUrl()
			seckill.SeckillPage()
			seckill.SubmitSeckillOrder()
		}(seckill)
	}
}

func preper(logger *zap.Logger, user jd_seckill.User, client *resty.Client) *jd_seckill.Seckill{
	userInfo, _ := user.GetUserInfo()
	logger.Info("用户:" + userInfo)
	
	// 开始预约,预约过的就重复预约
	seckill := jd_seckill.NewSeckill(client, conf.C, logger)
	seckill.MakeReserve()
	
	// 等待抢购/开始抢购
	nowLocalTime := time.Now().UnixNano() / 1e6
	jdTime, _ := getJdTime(logger)
	buyDate := conf.C.BuyTime
	loc, _ := time.LoadLocation("Local")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", buyDate, loc)
	buyTime := t.UnixNano() / 1e6
	diffTime := nowLocalTime - jdTime
	logger.Info(fmt.Sprintf("正在等待到达设定时间:%s，检测本地时间与京东服务器时间误差为【%d】毫秒", buyDate, diffTime))
	timerTime := (buyTime + diffTime) - jdTime
	// if timerTime <= 0 {
	// 	logger.Error("请设置抢购时间")
	// 	// TODO 这里先调试，关闭此处
	// 	// os.Exit(0)
	// }
	timerTime = 1000
	time.Sleep(time.Duration(timerTime) * time.Millisecond)
	return seckill
}
