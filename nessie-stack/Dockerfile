FROM jupyter/pyspark-notebook:latest

USER root

# Install AWS Hadoop integration for S3
RUN wget https://repo1.maven.org/maven2/org/apache/hadoop/hadoop-aws/3.3.4/hadoop-aws-3.3.4.jar -P /usr/local/spark/jars/ && \
    wget https://repo1.maven.org/maven2/com/amazonaws/aws-java-sdk-bundle/1.12.262/aws-java-sdk-bundle-1.12.262.jar -P /usr/local/spark/jars/

# Install Iceberg dependencies for Spark 3.5 (version 1.8.1)
RUN wget https://repo1.maven.org/maven2/org/apache/iceberg/iceberg-spark-runtime-3.5_2.12/1.8.1/iceberg-spark-runtime-3.5_2.12-1.8.1.jar -P /usr/local/spark/jars/ && \
    wget https://repo1.maven.org/maven2/org/projectnessie/nessie-integrations/nessie-spark-extensions-3.5_2.12/0.102.5/nessie-spark-extensions-3.5_2.12-0.102.5.jar -P /usr/local/spark/jars/

# Install Python packages (update pyiceberg to match JAR version)
RUN pip install pyiceberg==0.9.0 pynessie==0.67.0

# Create a directory for init scripts
RUN mkdir -p /usr/local/bin/start-notebook.d

# Create init script to configure Spark environment
RUN echo '#!/bin/bash\nexport PYSPARK_DRIVER_PYTHON=jupyter\nexport PYSPARK_DRIVER_PYTHON_OPTS="lab --NotebookApp.token=\'\' --NotebookApp.password=\'\'"' > /usr/local/bin/start-notebook.d/spark-config.sh && \
    chmod +x /usr/local/bin/start-notebook.d/spark-config.sh

USER $NB_UID

# Create a default spark-defaults.conf
RUN mkdir -p $HOME/.sparkmagic
COPY --chown=$NB_UID:$NB_GID spark-defaults.conf /usr/local/spark/conf/spark-defaults.conf