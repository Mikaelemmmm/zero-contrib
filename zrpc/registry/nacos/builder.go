package nacos

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/pkg/errors"
	"google.golang.org/grpc/resolver"
)

func init() {
	resolver.Register(&builder{})
}

// schemeName for the urls
// All target URLs like 'nacos://.../...' will be resolved by this resolver
const schemeName = "nacos"

// builder implements resolver.Builder and use for constructing all consul resolvers
type builder struct{}

func (b *builder) Build(url resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	dsn := strings.Join([]string{schemeName + ":/", url.Authority, url.Endpoint}, "/")
	tgt, err := parseURL(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "Wrong nacos URL")
	}

	host, ports, err := net.SplitHostPort(tgt.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing address error: %v", err)
	}
	port, _ := strconv.ParseUint(ports, 10, 16)

	sc := []constant.ServerConfig{
		*constant.NewServerConfig(host, port),
	}

	cli, err := clients.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: sc,
		ClientConfig: &constant.ClientConfig{
			Username:  tgt.User,
			Password:  tgt.Password,
			TimeoutMs: uint64(tgt.Timeout),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't connect to the nacos API")
	}

	ctx, cancel := context.WithCancel(context.Background())
	pipe := make(chan []string)

	go cli.Subscribe(&vo.SubscribeParam{
		ServiceName:       tgt.Service,
		Clusters:          tgt.Clusters,
		GroupName:         tgt.GroupName,
		SubscribeCallback: newWatcher(ctx, cancel, pipe).CallBackHandle, // required
	})

	go populateEndpoints(ctx, cc, pipe)

	return &resolvr{cancelFunc: cancel}, nil
}

// Scheme returns the scheme supported by this resolver.
// Scheme is defined at https://github.com/grpc/grpc/blob/master/doc/naming.md.
func (b *builder) Scheme() string {
	return schemeName
}
