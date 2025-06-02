#!/usr/bin/env bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}错误：${plain} 必须使用root用户运行此脚本！\n" && exit 1

# check os
if [[ -f /etc/redhat-release ]]; then
    release="centos"
elif cat /etc/issue | grep -Eqi "debian"; then
    release="debian"
elif cat /etc/issue | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /etc/issue | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
elif cat /proc/version | grep -Eqi "debian"; then
    release="debian"
elif cat /proc/version | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /proc/version | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
else
    echo -e "${red}未检测到系统版本，请联系脚本作者！${plain}\n" && exit 1
fi

arch=$(arch)

if [[ $arch == "x86_64" || $arch == "x64" || $arch == "s390x" || $arch == "amd64" ]]; then
    arch="amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    arch="arm64"
else
    arch="amd64"
    echo -e "${red}检测架构失败，使用默认架构: ${arch}${plain}"
fi

echo "架构: ${arch}"

if [ $(getconf WORD_BIT) != '32' ] && [ $(getconf LONG_BIT) != '64' ]; then
    echo "本软件不支持 32 位系统(x86)，请使用 64 位系统(x86_64)，如果检测有误，请联系作者"
    exit -1
fi

os_version=""

# os version
if [[ -f /etc/os-release ]]; then
    os_version=$(awk -F'[= ."]' '/VERSION_ID/{print $3}' /etc/os-release)
fi
if [[ -z "$os_version" && -f /etc/lsb-release ]]; then
    os_version=$(awk -F'[= ."]+' '/DISTRIB_RELEASE/{print $2}' /etc/lsb-release)
fi

if [[ x"${release}" == x"centos" ]]; then
    if [[ ${os_version} -le 6 ]]; then
        echo -e "${red}请使用 CentOS 7 或更高版本的系统！${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"ubuntu" ]]; then
    if [[ ${os_version} -lt 16 ]]; then
        echo -e "${red}请使用 Ubuntu 16 或更高版本的系统！${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"debian" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red}请使用 Debian 8 或更高版本的系统！${plain}\n" && exit 1
    fi
fi

install_base() {
    echo -e "${yellow}开始安装基础组件...${plain}"
    if [[ x"${release}" == x"centos" ]]; then
        yum clean all
        yum makecache
        yum install wget curl tar jq socat -y
    else
        apt update -y
        apt install wget curl tar jq socat -y
    fi
    if [[ $? -ne 0 ]]; then
        echo -e "${red}基础组件安装失败，请检查网络或手动安装 wget curl tar jq socat${plain}"
        exit 1
    fi
    echo -e "${green}基础组件安装完成${plain}"
}

check_config() {
    if [[ ! -e /etc/x-ui ]]; then
        mkdir /etc/x-ui
    fi
}

check_xray() {
    if [[ ! -f /usr/local/x-ui/bin/xray-linux-${arch} ]]; then
        echo -e "${red}未找到 Xray 可执行文件，安装可能已损坏，请尝试重新安装${plain}"
        exit 1
    fi
}

config_after_install() {
    echo -e "${yellow}出于安全考虑，安装/更新完成后需要强制修改端口与账户密码${plain}"
    read -p "确认是否继续,如选择n则跳过本次端口与账户密码设定[y/n]: " config_confirm
    if [[ x"${config_confirm}" == x"y" || x"${config_confirm}" == x"Y" ]]; then
        while true; do
            read -p "请设置您的账户名(至少4位): " config_account
            if [[ ${#config_account} -ge 4 ]]; then
                break
            else
                echo -e "${red}账户名长度不能小于4位${plain}"
            fi
        done
        echo -e "${yellow}您的账户名将设定为: ${config_account}${plain}"
        
        while true; do
            read -p "请设置您的账户密码(至少6位): " config_password
            if [[ ${#config_password} -ge 6 ]]; then
                break
            else
                echo -e "${red}密码长度不能小于6位${plain}"
            fi
        done
        echo -e "${yellow}您的账户密码将设定为: ${config_password}${plain}"
        
        while true; do
            read -p "请设置面板访问端口(1-65535): " config_port
            if [[ ${config_port} -ge 1 && ${config_port} -le 65535 ]]; then
                break
            else
                echo -e "${red}端口范围错误，请输入1-65535之间的数字${plain}"
            fi
        done
        echo -e "${yellow}您的面板访问端口将设定为: ${config_port}${plain}"
        
        echo -e "${yellow}确认设定,设定中...${plain}"
        /usr/local/x-ui/x-ui setting -username ${config_account} -password ${config_password}
        if [[ $? -eq 0 ]]; then
            echo -e "${green}账户密码设定完成${plain}"
        else
            echo -e "${red}账户密码设定失败${plain}"
            exit 1
        fi
        
        /usr/local/x-ui/x-ui setting -port ${config_port}
        if [[ $? -eq 0 ]]; then
            echo -e "${green}面板端口设定完成${plain}"
        else
            echo -e "${red}面板端口设定失败${plain}"
            exit 1
        fi
    else
        echo -e "${yellow}跳过手动设定，将使用随机生成的配置...${plain}"
        if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then
            local usernameTemp=$(head -c 8 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 8)
            local passwordTemp=$(head -c 12 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 12)
            local portTemp=$(shuf -i 10000-65535 -n 1)
            
            /usr/local/x-ui/x-ui setting -username ${usernameTemp} -password ${passwordTemp}
            /usr/local/x-ui/x-ui setting -port ${portTemp}
            
            echo -e "检测到您属于全新安装,已自动为您生成随机配置:"
            echo -e "###############################################"
            echo -e "${green}面板登录用户名: ${usernameTemp}${plain}"
            echo -e "${green}面板登录密码: ${passwordTemp}${plain}"
            echo -e "${green}面板访问端口: ${portTemp}${plain}"
            echo -e "###############################################"
            echo -e "${yellow}请务必保存好以上信息！${plain}"
            echo -e "${yellow}您可以稍后使用x-ui命令,选择选项7查看面板登录信息${plain}"
        else
            echo -e "${yellow}检测到是版本升级，将保留原有配置${plain}"
            echo -e "${yellow}如需查看配置信息，请使用x-ui命令并选择选项7${plain}"
        fi
    fi
}

install_x-ui() {
    systemctl stop x-ui
    cd /usr/local/

    if [ $# == 0 ]; then
        echo -e "${yellow}开始检查最新版本...${plain}"
        last_version=$(curl -Ls "https://api.github.com/repos/FranzKafkaYu/x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}检测版本失败，请检查网络或稍后再试${plain}"
            exit 1
        fi
        echo -e "${green}检测到最新版本：${last_version}${plain}"
        
        echo -e "${yellow}开始下载 x-ui...${plain}"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz https://github.com/FranzKafkaYu/x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载失败，请检查网络或手动下载${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/FranzKafkaYu/x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz"
        echo -e "${yellow}开始下载 x-ui v${last_version}...${plain}"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 v${last_version} 失败，请检查版本是否存在${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        rm /usr/local/x-ui/ -rf
    fi

    echo -e "${yellow}开始解压安装包...${plain}"
    tar zxvf x-ui-linux-${arch}.tar.gz
    rm x-ui-linux-${arch}.tar.gz -f
    cd x-ui
    chmod +x x-ui bin/xray-linux-${arch}
    cp -f x-ui.service /etc/systemd/system/
    
    echo -e "${yellow}开始下载脚本文件...${plain}"
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/875706361/x-ui_he/master/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}下载脚本失败，请检查网络${plain}"
        exit 1
    fi
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    
    echo -e "${yellow}开始配置x-ui...${plain}"
    config_after_install
    
    echo -e "${yellow}开始启动服务...${plain}"
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    
    echo -e "${green}x-ui v${last_version} 安装完成，面板已启动${plain}"
    echo -e ""
    echo -e "x-ui 管理脚本使用方法: "
    echo -e "----------------------------------------------"
    echo -e "x-ui              - 显示管理菜单 (功能更多)"
    echo -e "x-ui start        - 启动 x-ui 面板"
    echo -e "x-ui stop         - 停止 x-ui 面板"
    echo -e "x-ui restart      - 重启 x-ui 面板"
    echo -e "x-ui status       - 查看 x-ui 状态"
    echo -e "x-ui enable       - 设置 x-ui 开机自启"
    echo -e "x-ui disable      - 取消 x-ui 开机自启"
    echo -e "x-ui log          - 查看 x-ui 日志"
    echo -e "x-ui update       - 更新 x-ui 面板"
    echo -e "x-ui install      - 安装 x-ui 面板"
    echo -e "x-ui uninstall    - 卸载 x-ui 面板"
    echo -e "x-ui geo          - 更新 geo 数据"
    echo -e "----------------------------------------------"
}

echo -e "${green}开始安装x-ui面板...${plain}"
install_base
check_config
install_x-ui $1
check_xray
