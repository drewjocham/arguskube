#!/bin/bash
set -euo pipefail

FLINK_VERSION="${flink_version}"
GATEWAY_PORT="${gateway_port}"
API_KEY="${api_key}"
KAFKA_SERVERS="${kafka_servers}"

# Install dependencies
apt-get update
apt-get install -y openjdk-11-jre-headless python3 python3-pip curl wget

# Install Apache Flink
cd /opt
wget -q "https://dlcdn.apache.org/flink/flink-${FLINK_VERSION}/flink-${FLINK_VERSION}-bin-scala_2.12.tgz"
tar xzf "flink-${FLINK_VERSION}-bin-scala_2.12.tgz"
ln -s "/opt/flink-${FLINK_VERSION}" /opt/flink

# Configure Flink for single-node cluster
cat >> /opt/flink/conf/flink-conf.yaml << EOF
jobmanager.rpc.address: localhost
jobmanager.memory.process.size: 2g
taskmanager.memory.process.size: 4g
taskmanager.numberOfTaskSlots: 4
parallelism.default: 2
rest.port: 8081
EOF

# Install Flink Kafka connector
cd /opt/flink
wget -q "https://repo.maven.apache.org/maven2/org/apache/flink/flink-sql-connector-kafka/1.18.0/flink-sql-connector-kafka-1.18.0.jar" -O lib/flink-sql-connector-kafka.jar

# Install Python dependencies for PyFlink job
pip3 install pyflink==${FLINK_VERSION} apache-flink==${FLINK_VERSION}

# Create systemd service for Flink JobManager
cat > /etc/systemd/system/flink-jobmanager.service << EOF
[Unit]
Description=Apache Flink JobManager
After=network.target

[Service]
Type=simple
User=flink
Group=flink
ExecStart=/opt/flink/bin/jobmanager.sh start-foreground
Restart=on-failure
RestartSec=10
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# Create systemd service for Flink TaskManager
cat > /etc/systemd/system/flink-taskmanager.service << EOF
[Unit]
Description=Apache Flink TaskManager
After=flink-jobmanager.service

[Service]
Type=simple
User=flink
Group=flink
ExecStart=/opt/flink/bin/taskmanager.sh start-foreground
Restart=on-failure
RestartSec=10
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# Create flink user
useradd -r -s /bin/false flink
chown -R flink:flink /opt/flink

# Start Flink services
systemctl daemon-reload
systemctl enable flink-jobmanager flink-taskmanager
systemctl start flink-jobmanager flink-taskmanager

# Deploy Flink gateway as Docker container
apt-get install -y docker.io
docker pull ghcr.io/drewjocham/kubewatcher-flink-gateway:latest
docker run -d \
    --name flink-gateway \
    --restart always \
    -p ${GATEWAY_PORT}:${GATEWAY_PORT} \
    -e FLINK_URL=http://localhost:8081 \
    -e GATEWAY_PORT=${GATEWAY_PORT} \
    -e GATEWAY_API_KEY=${API_KEY} \
    ghcr.io/drewjocham/kubewatcher-flink-gateway:latest

echo "Flink deployment complete"
