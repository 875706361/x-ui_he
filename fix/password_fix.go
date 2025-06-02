package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 用户模型，保持与3x-ui一致
type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: password_fix <数据库路径>")
		fmt.Println("例如: password_fix /etc/x-ui/x-ui.db")
		return
	}

	dbPath := os.Args[1]

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("数据库文件不存在: %s", dbPath)
	}

	// 创建备份
	backupPath := dbPath + ".bak." + fmt.Sprint(os.Getpid())
	data, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("读取数据库文件失败: %v", err)
	}

	err = os.WriteFile(backupPath, data, 0644)
	if err != nil {
		log.Fatalf("创建数据库备份失败: %v", err)
	}

	fmt.Printf("已创建备份: %s\n", backupPath)

	// 打开数据库
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	// 查询所有用户
	var users []User
	err = db.Find(&users).Error
	if err != nil {
		log.Fatalf("查询用户失败: %v", err)
	}

	fmt.Printf("找到 %d 个用户\n", len(users))

	// 逐个处理用户密码
	for _, user := range users {
		fmt.Printf("处理用户 %s (ID: %d)\n", user.Username, user.Id)

		// 简单设置为admin/admin，或者用户可以根据需要提供新密码
		// 这只是一个临时解决方案
		user.Password = "admin"

		// 更新用户密码
		err = db.Model(&User{}).Where("id = ?", user.Id).Update("password", user.Password).Error
		if err != nil {
			log.Fatalf("更新用户 %s 的密码失败: %v", user.Username, err)
		}

		fmt.Printf("已将用户 %s 的密码重置为 'admin'\n", user.Username)
	}

	fmt.Println("密码修复完成！")
	fmt.Println("注意：所有用户密码已重置为 'admin'")
	fmt.Printf("备份文件路径: %s\n", backupPath)
}
