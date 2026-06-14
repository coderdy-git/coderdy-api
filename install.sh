#!/map/bash

# WhatsApp Gateway Auto Installer
# Jalankan dengan: sudo bash install.sh

echo "==== Memulai Instalasi WA Gateway ===="

# 1. Update Sistem
sudo apt update && sudo apt upgrade -y

# 2. Cek/Install Go
if ! command -v go &> /dev/null
then
    echo "Golang tidak ditemukan, menginstall Golang..."
    wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    rm go1.22.4.linux-amd64.tar.gz
fi

# 3. Setup Project Folder
WORKDIR=$(pwd)
echo "Menggunakan direktori: $WORKDIR"

# 4. Install Dependencies & Build
echo "Mengunduh dependencies..."
go mod tidy
echo "Melakukan Build Binary..."
go build -o wa-gateway main.go

# 5. Membuat Systemd Service
echo "Membuat Systemd Service..."
sudo bash -c "cat <<EOT > /etc/systemd/system/wa-gateway.service
[Unit]
Description=WhatsApp Gateway Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$WORKDIR
ExecStart=$WORKDIR/wa-gateway
Restart=always
RestartSec=5
EnvironmentFile=$WORKDIR/.env

[Install]
WantedBy=multi-user.target
EOT"

# 6. Aktifkan Service
sudo systemctl daemon-reload
sudo systemctl enable wa-gateway

echo "==== Instalasi Selesai ===="
echo "Langkah Selanjutnya:"
echo "1. Edit file .env dan masukkan API Key Anda."
echo "2. Jalankan perintah: sudo systemctl start wa-gateway"
echo "3. Cek status dengan: sudo systemctl status wa-gateway"
