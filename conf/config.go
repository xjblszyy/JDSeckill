package conf

import (
	"os"
	"path/filepath"

	"github.com/jinzhu/configor"
	"go.uber.org/zap"

	"JDSeckill/service/email"
)

type Config struct {
	Debug  bool       `toml:"debug"`
	Logger LoggerConf `toml:"logger"`
	EID    string     `toml:"eid"`
	// eid, fp参数必须填写，具体请参考 wiki-常见问题
	FP string `toml:"fp"`
	// 商品id
	// 已经是茅台的sku_id了
	SkuId string `toml:"sku_id" default:"100012043978"`
	// 抢购数量
	SeckillNum int `toml:"seckill_num" default:"1"`
	// 设定时间 # 2020-12-09 10:00:00.100000
	BuyTime string `toml:"buy_time"`
	// 默认UA
	UserAgent string `toml:"user_agent"`
	// 支付密码
	Account AccountConf `toml:"account"`
	// 消息推送
	Message MessageConf `toml:"message"`
	// smtp配置
	// 开启smtp消息推送必须填入 email_user、email_pwd，email_host 若不填则会自动判断，email_pwd 如何获取请自行百度。
	Smtp email.Config `toml:"smtp"`
}

type LoggerConf struct {
	Env   string `toml:"env" default:"prod"`
	Level string `toml:"level" default:"info"`
}

type AccountConf struct {
	// 如果你的账户中有可用的京券（注意不是东券）或 在上次购买订单中使用了京豆，
	// 那么京东可能会在下单时自动选择京券支付 或 自动勾选京豆支付。
	// 此时下单会要求输入六位数字的支付密码。请在下方配置你的支付密码，如 123456 。
	// 如果没有上述情况，下方请留空。
	PaymentPwd string `toml:"payment_pwd"`
}

type MessageConf struct {
	// 开启推送服务
	Enable bool `toml:"enable"`
	// 目前只支持smtp邮箱推送
	Type string `toml:"type"`
	// 消息接收人
	Email string `toml:"email"`
}

var C = &Config{}

// 读取配置
func ParseConfig(cfgFile string) {
	if cfgFile != "" {
		if err := configor.New(&configor.Config{AutoReload: true}).Load(C, cfgFile); err != nil {
			zap.L().Panic("init config fail", zap.Error(err))
		}
	} else {
		if err := configor.New(&configor.Config{AutoReload: true}).Load(C); err != nil {
			zap.L().Panic("init config fail", zap.Error(err))
		}
	}

	zapLevel := zap.NewAtomicLevel()
	if err := zapLevel.UnmarshalText([]byte(C.Logger.Level)); err != nil {
		panic(err)
	}

	var zapConf zap.Config
	if env := C.Logger.Env; env == "dev" {
		zapConf = zap.NewDevelopmentConfig()
	} else {
		zapConf = zap.NewProductionConfig()
	}
	zapConf.Level = zapLevel

	if logger, err := zapConf.Build(zap.Fields(zap.String("proc", filepath.Base(os.Args[0])))); err != nil {
		panic(err)
	} else {
		zap.RedirectStdLog(logger)
		zap.ReplaceGlobals(logger)
	}
	zap.L().Named("configor").Info("loaded config")
}
