#!/bin/bash

# 获取脚本所在的文件夹
script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# 设置服务文件路径
service_file="/etc/systemd/system/smoothserve.service"

# 设置 smoothserve_linux 路径和工作目录路径
executable_path="$script_dir/smoothserve"
working_directory="$script_dir"

# 创建服务文件
echo "[Unit]" > $service_file
echo "Description=Smooth Serve Service" >> $service_file
echo "After=network.target" >> $service_file
echo "" >> $service_file
echo "[Service]" >> $service_file
echo "Type=simple" >> $service_file
echo "ExecStart=$executable_path" >> $service_file
echo "WorkingDirectory=$working_directory" >> $service_file
echo "Restart=always" >> $service_file
echo "" >> $service_file
echo "[Install]" >> $service_file
echo "WantedBy=multi-user.target" >> $service_file

# 重新加载 systemd 配置
systemctl daemon-reload

# 启动服务并设置为开机自启
systemctl start smoothserve
systemctl enable smoothserve

echo "Smooth Serve installed."
