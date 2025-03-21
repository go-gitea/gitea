# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# syntax=docker/dockerfile:1
FROM python:3.10-bullseye

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      sudo \
      curl \
      vim \
      unzip \
      openjdk-17-jdk \
      build-essential \
      software-properties-common \
      ssh && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Jupyter and other python deps
COPY requirements.txt .
RUN pip3 install -r requirements.txt

# Add scala kernel via spylon-kernel
RUN python3 -m spylon_kernel install

# Download and install IJava jupyter kernel
RUN curl https://github.com/SpencerPark/IJava/releases/download/v1.3.0/ijava-1.3.0.zip -Lo ijava-1.3.0.zip \
  && unzip ijava-1.3.0.zip \
  && python3 install.py --sys-prefix \
  && rm ijava-1.3.0.zip

# Optional env variables
ENV SPARK_HOME=${SPARK_HOME:-"/opt/spark"}
ENV PYTHONPATH=$SPARK_HOME/python:$SPARK_HOME/python/lib/py4j-0.10.9.7-src.zip:$PYTHONPATH

WORKDIR ${SPARK_HOME}

ENV SPARK_VERSION=3.5.2
ENV SPARK_MAJOR_VERSION=3.5
ENV ICEBERG_VERSION=1.8.1
ENV NESSIE_VERSION=0.103.2
# Download spark
RUN mkdir -p ${SPARK_HOME} \
 && curl https://archive.apache.org/dist/spark/spark-${SPARK_VERSION}/spark-${SPARK_VERSION}-bin-hadoop3.tgz -o spark-${SPARK_VERSION}-bin-hadoop3.tgz \
 && tar xvzf spark-${SPARK_VERSION}-bin-hadoop3.tgz --directory /opt/spark --strip-components 1 \
 && rm -rf spark-${SPARK_VERSION}-bin-hadoop3.tgz

RUN curl https://repo1.maven.org/maven2/org/projectnessie/nessie-integrations/nessie-spark-extensions-${SPARK_MAJOR_VERSION}_2.12/${NESSIE_VERSION}/nessie-spark-extensions-${SPARK_MAJOR_VERSION}_2.12-${NESSIE_VERSION}.jar -Lo /opt/spark/jars/nessie-spark-extensions-${SPARK_MAJOR_VERSION}_2.12-${NESSIE_VERSION}.jar

# Download iceberg spark runtime
RUN curl https://repo1.maven.org/maven2/org/apache/iceberg/iceberg-spark-runtime-${SPARK_MAJOR_VERSION}_2.12/${ICEBERG_VERSION}/iceberg-spark-runtime-${SPARK_MAJOR_VERSION}_2.12-${ICEBERG_VERSION}.jar -Lo /opt/spark/jars/iceberg-spark-runtime-${SPARK_MAJOR_VERSION}_2.12-${ICEBERG_VERSION}.jar

# Download AWS bundle
RUN curl -s https://repo1.maven.org/maven2/org/apache/iceberg/iceberg-aws-bundle/${ICEBERG_VERSION}/iceberg-aws-bundle-${ICEBERG_VERSION}.jar -Lo /opt/spark/jars/iceberg-aws-bundle-${ICEBERG_VERSION}.jar

# Download GCP bundle
RUN curl -s https://repo1.maven.org/maven2/org/apache/iceberg/iceberg-gcp-bundle/${ICEBERG_VERSION}/iceberg-gcp-bundle-${ICEBERG_VERSION}.jar -Lo /opt/spark/jars/iceberg-gcp-bundle-${ICEBERG_VERSION}.jar

# Download Azure bundle
RUN curl -s https://repo1.maven.org/maven2/org/apache/iceberg/iceberg-azure-bundle/${ICEBERG_VERSION}/iceberg-azure-bundle-${ICEBERG_VERSION}.jar -Lo /opt/spark/jars/iceberg-azure-bundle-${ICEBERG_VERSION}.jar

# Install AWS CLI
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
 && unzip awscliv2.zip \
 && sudo ./aws/install \
 && rm awscliv2.zip \
 && rm -rf aws/

# Add iceberg spark runtime jar to IJava classpath
ENV IJAVA_CLASSPATH=/opt/spark/jars/*

RUN mkdir -p /home/iceberg/data \
 && curl https://data.cityofnewyork.us/resource/tg4x-b46p.json > /home/iceberg/data/nyc_film_permits.json \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2022-04.parquet -o /home/iceberg/data/yellow_tripdata_2022-04.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2022-03.parquet -o /home/iceberg/data/yellow_tripdata_2022-03.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2022-02.parquet -o /home/iceberg/data/yellow_tripdata_2022-02.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2022-01.parquet -o /home/iceberg/data/yellow_tripdata_2022-01.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-12.parquet -o /home/iceberg/data/yellow_tripdata_2021-12.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-11.parquet -o /home/iceberg/data/yellow_tripdata_2021-11.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-10.parquet -o /home/iceberg/data/yellow_tripdata_2021-10.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-09.parquet -o /home/iceberg/data/yellow_tripdata_2021-09.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-08.parquet -o /home/iceberg/data/yellow_tripdata_2021-08.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-07.parquet -o /home/iceberg/data/yellow_tripdata_2021-07.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-06.parquet -o /home/iceberg/data/yellow_tripdata_2021-06.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-05.parquet -o /home/iceberg/data/yellow_tripdata_2021-05.parquet \
 && curl https://d37ci6vzurychx.cloudfront.net/trip-data/yellow_tripdata_2021-04.parquet -o /home/iceberg/data/yellow_tripdata_2021-04.parquet

RUN mkdir -p /home/iceberg/localwarehouse /home/iceberg/notebooks /home/iceberg/warehouse /home/iceberg/spark-events /home/iceberg
COPY notebooks/ /home/iceberg/notebooks

# Add a notebook command
RUN echo '#! /bin/sh' >> /bin/notebook \
 && echo 'export PYSPARK_DRIVER_PYTHON=jupyter-notebook' >> /bin/notebook \
 && echo "export PYSPARK_DRIVER_PYTHON_OPTS=\"--notebook-dir=/home/iceberg/notebooks --ip='*' --NotebookApp.token='' --NotebookApp.password='' --port=8888 --no-browser --allow-root\"" >> /bin/notebook \
 && echo "pyspark" >> /bin/notebook \
 && chmod u+x /bin/notebook

# Add a pyspark-notebook command (alias for notebook command for backwards-compatibility)
RUN echo '#! /bin/sh' >> /bin/pyspark-notebook \
 && echo 'export PYSPARK_DRIVER_PYTHON=jupyter-notebook' >> /bin/pyspark-notebook \
 && echo "export PYSPARK_DRIVER_PYTHON_OPTS=\"--notebook-dir=/home/iceberg/notebooks --ip='*' --NotebookApp.token='' --NotebookApp.password='' --port=8888 --no-browser --allow-root\"" >> /bin/pyspark-notebook \
 && echo "pyspark" >> /bin/pyspark-notebook \
 && chmod u+x /bin/pyspark-notebook

RUN mkdir -p /root/.ipython/profile_default/startup
COPY ipython/startup/00-prettytables.py /root/.ipython/profile_default/startup
COPY ipython/startup/README /root/.ipython/profile_default/startup

COPY spark-defaults.conf /opt/spark/conf
ENV PATH="/opt/spark/sbin:/opt/spark/bin:${PATH}"

RUN chmod u+x /opt/spark/sbin/* && \
    chmod u+x /opt/spark/bin/*

COPY .pyiceberg.yaml /root/.pyiceberg.yaml

COPY entrypoint.sh .

ENTRYPOINT ["./entrypoint.sh"]
CMD ["notebook"]
