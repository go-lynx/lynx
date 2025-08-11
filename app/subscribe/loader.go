package subscribe

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/conf"
	ggrpc "google.golang.org/grpc"
)

// BuildGrpcSubscriptions 依据配置构建 gRPC 订阅连接
// 返回值 key 为服务名，value 为可复用的 gRPC ClientConn
func BuildGrpcSubscriptions(cfg *conf.Subscriptions, discovery registry.Discovery, routerFactory func(string) selector.NodeFilter) (map[string]*ggrpc.ClientConn, error) {
	conns := make(map[string]*ggrpc.ClientConn)
	if cfg == nil || len(cfg.GetGrpc()) == 0 {
		return conns, nil
	}

	for _, item := range cfg.GetGrpc() {
		name := item.GetService()
		if name == "" {
			log.Warnf("skip empty grpc subscription entry")
			continue
		}

		opts := []Option{
			WithServiceName(name),
			WithDiscovery(discovery),
		}
		if routerFactory != nil {
			opts = append(opts, WithNodeRouterFactory(routerFactory))
		}
		if item.GetTls() {
			opts = append(opts, EnableTls())
		}
		if cn := item.GetCaName(); cn != "" {
			opts = append(opts, WithRootCAFileName(cn))
		}
		if cg := item.GetCaGroup(); cg != "" {
			opts = append(opts, WithRootCAFileGroup(cg))
		}
		if item.GetRequired() {
			opts = append(opts, Required())
		}

		sub := NewGrpcSubscribe(opts...)
		conn := sub.Subscribe()
		if conn == nil {
			if item.GetRequired() {
				return nil, fmt.Errorf("required grpc subscription failed: %s", name)
			}
			log.Warnf("grpc subscription created nil conn: %s", name)
			continue
		}

		// 预热：简单执行连接状态检查（可选）
		state := conn.GetState()
		log.Infof("grpc subscription established: service=%s state=%s", name, state.String())

		conns[name] = conn
	}

	return conns, nil
}

// AsDialOption 可选：将已存在连接装配为 kratos 客户端选项（按需使用）
func AsDialOption(conn *ggrpc.ClientConn) kratosgrpc.ClientOption {
	return kratosgrpc.WithConn(conn)
}
