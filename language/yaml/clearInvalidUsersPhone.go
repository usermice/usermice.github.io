package main
# go 语言范本
import (
        "bufio"
        "fmt"
        "gauth"
        "os"
        "strconv"
        "strings"
        "time"
        "wng/common/appx"
)

// 清除未实名，绑定号段为165/170/171的用户手机号
type UserInfo struct {
        UserId      int32 `db:"user_id"`
        PhoneNumber int64 `db:"phone_number"`
}

func main() {
        pub := make([]byte, 32)
        key := make([]byte, 32)
        gauth.Init(1, "backstage", "127.0.0.1:8080", pub, key)
        defer gauth.Quit()
        ctx := appx.Background("backstage")

        fmt.Println("开始执行，请稍等")

        sql := `SELECT users.user_id, users.phone_number 
        FROM users
        LEFT JOIN wallet_owner_identity ON users.user_id = wallet_owner_identity.user_id
        WHERE (wallet_owner_identity.user_id IS NULL OR wallet_owner_identity.certification_time = 0)
        AND users.phone_number IS NOT NULL`

        db, _ := appx.GetDatabase(ctx)
        rows, err := db.Query(sql)
        if err != nil {
                fmt.Println("查询错误:", err)
                return
        }
        defer rows.Close()

        usersToUpdate := make([]UserInfo, 0, 1024*16)

        for rows.Next() {
                var user UserInfo
                err = rows.StructScan(&user)
                if err != nil {
                        fmt.Println("获取数据错误:", err)
                        return
                }
                plainPhoneNumber, _ := gauth.DecryptPhone(uint64(user.PhoneNumber))
                str := strconv.FormatUint(plainPhoneNumber, 10)
                if strings.HasPrefix(str, "165") || strings.HasPrefix(str, "170") || strings.HasPrefix(str, "171") {
                        usersToUpdate = append(usersToUpdate, user)
                }
        }

        count := len(usersToUpdate)
        fmt.Printf("总共找到 %d 条记录。确认继续？(y/n): ", count)
        scanner := bufio.NewScanner(os.Stdin)
        scanner.Scan()
        response := scanner.Text()
        if response != "y" {
                fmt.Println("操作已取消")
                return
        }

        currentTime := time.Now()
        timeStamp := currentTime.Format("20060102-150405")
        fileName := fmt.Sprintf("restore_updates_%s.sql", timeStamp)
        file, err := os.Create(fileName)
        if err != nil {
                fmt.Println("无法创建操作还原文件:", err)
                return
        }
        defer file.Close()

        processed := 0
        for _, user := range usersToUpdate {
                processed++
                percentage := float64(processed) / float64(count) * 100

                sqlUpdate := fmt.Sprintf("update users set phone_number=NULL where user_id=%d", user.UserId)
                sqlRestore := fmt.Sprintf("update users set phone_number=%d where user_id=%d;\n", user.PhoneNumber, user.UserId)
                _, err := file.WriteString(sqlRestore)
                if err != nil {
                        fmt.Println("写入备份文件错误:", err)
                        return
                }
                db.Execute(sqlUpdate)
                fmt.Printf("\r完成进度：%.0f%%", percentage)
        }
        fmt.Println("\n执行完成")
}
