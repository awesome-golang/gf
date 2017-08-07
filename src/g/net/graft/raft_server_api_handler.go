// 返回格式统一：
// {result:1, message:"", data:""}

package graft

import (
    "strings"
    "g/net/ghttp"
    "g/encoding/gjson"
    "time"
)


// K-V API管理
func (n *Node) kvApiHandler(r *ghttp.Request, w *ghttp.Response) {
    method := strings.ToUpper(r.Method)
    switch method {
        case "GET":
            k := r.GetRequestString("k")
            if k == "" {
                w.ResponseJson(1, "ok", *n.KVMap.Clone())
            } else {
                w.ResponseJson(1, "ok", n.KVMap.Get(k))
            }

        case "PUT":
            fallthrough
        case "POST":
            fallthrough
        case "DELETE":
            data := r.GetRaw()
            if data == "" {
                w.ResponseJson(0, "invalid input", nil)
                return
            }
            var items interface{}
            err := gjson.DecodeTo(&data, &items)
            if err != nil {
                w.ResponseJson(0, "invalid data type: " + err.Error(), nil)
                return
            }
            // 只允许map[string]interface{}和[]interface{}两种数据类型
            isSSMap  := isStringStringMap(items)
            isSArray := isStringArray(items)
            if !(isSSMap && (method == "PUT" || method == "POST")) && !(isSArray && method == "DELETE")  {
                w.ResponseJson(0, "invalid data type for " + method, nil)
                return
            }
            // 请求到leader
            conn := n.getConn(n.getLeader(), gPORT_REPL)
            if conn == nil {
                w.ResponseJson(0, "could not connect to leader: " + n.getLeader(), nil)
                return
            }
            head := gMSG_HEAD_SET
            if method == "DELETE" {
                head = gMSG_HEAD_REMOVE
            }
            err   = n.sendMsg(conn, head, *gjson.Encode(items))
            if err != nil {
                w.ResponseJson(0, "sending request error: " + err.Error(), nil)
            } else {
                msg := n.receiveMsg(conn)
                if msg.Head != gMSG_HEAD_LOG_REPL_RESPONSE {
                    w.ResponseJson(0, "handling request error", nil)
                } else {
                    w.ResponseJson(1, "ok", nil)
                }
            }
            conn.Close()
    }
}

// 节点信息API管理
func (n *Node) nodeApiHandler(r *ghttp.Request, w *ghttp.Response) {
    method := strings.ToUpper(r.Method)
    switch method {
        case "GET":
            conn := n.getConn(n.getLeader(), gPORT_RAFT)
            if conn == nil {
                w.ResponseJson(0, "could not connect to leader: " + n.getLeader(), nil)
                return
            }
            err := n.sendMsg(conn, gMSG_HEAD_PEERS_INFO, "")
            if err != nil {
                w.ResponseJson(0, "sending request error: " + err.Error(), nil)
            } else {
                var data interface{}
                msg := n.receiveMsg(conn)
                err  = gjson.DecodeTo(&msg.Body, &data)
                if err != nil {
                    w.ResponseJson(0, "received error from leader: " + err.Error(), nil)
                } else {
                    list := make([]NodeInfo, 0)
                    list  = append(list, NodeInfo {
                        Name          : msg.From.Name,
                        Ip            : n.getLeader(),
                        LastLogId     : msg.From.LastLogId,
                        LogCount      : msg.From.LogCount,
                        LastHeartbeat : time.Now().String(),
                    })
                    for _, v := range data.(map[string]interface{}) {
                        list = append(list, v.(NodeInfo))
                    }
                    w.ResponseJson(1, "ok", list)
                }
            }
            conn.Close()

        default:
            w.ResponseJson(0, "unsupported method " + method, nil)
    }
}

// 判断是否为map[string]string类型
func isStringStringMap(val interface{}) bool {
    switch val.(type) {
        case map[string]interface{}:
            for _, v := range val.(map[string]interface{}) {
                switch v.(type) {
                    case string:
                    default:
                        return false
                }
            }
        default:
            return false
    }
    return true
}

// 判断是否为[]string类型
func isStringArray(val interface{}) bool {
    switch val.(type) {
        case []interface{}:
            for _, v := range val.([]interface{}) {
                switch v.(type) {
                    case string:
                    default:
                        return false
                }
            }
        default:
            return false
    }
    return true
}

