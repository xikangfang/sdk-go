package metrics

import (
	"net"
	"sort"
	"strings"
)

// transfer TagKvs map to string in fixed order
func processTags(tagKvs map[string]string) string {
	tagStrs := make([]string, 0, len(tagKvs))
	for tagKey, tagValue := range tagKvs {
		tagStrs = append(tagStrs, tagKey+"="+tagValue)
	}
	sort.Strings(tagStrs)
	return strings.Join(tagStrs, "|")
}

// recover tagString to origin TagKvs map
func recoverTags(tagString string) map[string]string {
	tagKvs := make(map[string]string)
	kvs := strings.Split(tagString, "|")
	for _, kv := range kvs {
		res := strings.Split(kv, "=")
		if len(res) != 2 {
			continue
		}
		tagKvs[res[0]] = res[1]
	}
	return tagKvs
}

// build new TagKvs with baseTags and tagStrs
func appendTags(baseTags map[string]string, tagStrs []string) map[string]string {
	tagKvs := make(map[string]string)
	for k, v := range baseTags {
		tagKvs[k] = v
	}
	for _, tagStr := range tagStrs {
		res := strings.Split(tagStr, ":")
		if len(res) != 2 {
			continue
		}
		tagKvs[res[0]] = res[1]
	}
	return tagKvs
}

func getLocalHost() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.To4().String()
			}
		}
	}
	return ""
}

func StringMapClone(src map[string]string) map[string]string {
	dest := make(map[string]string, len(src))
	for k, v := range src {
		dest[k] =v
	}
	return dest
}


