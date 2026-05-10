FROM flink:1.18-scala_2.12-java11

# Install Python for PyFlink
RUN apt-get update && \
    apt-get install -y python3 python3-pip python3-dev && \
    pip3 install --no-cache-dir pyflink==1.18.0 apache-flink==1.18.0 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy Flink job
COPY job/ /opt/flink/jobs/

# Setup kubewatcher-specific Flink config
RUN mkdir -p /opt/flink/plugins/kafka && \
    mkdir -p /opt/flink/lib

# Save this as a job that can be submitted
ENV FLINK_PROPERTIES="jobmanager.rpc.address: localhost"

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8081/overview || exit 1

EXPOSE 8081 6123
