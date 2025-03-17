
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

from prettytable import PrettyTable
from IPython.core.magic import register_line_cell_magic

class DFTable(PrettyTable):
    def __repr__(self):
        return self.get_string()

    def _repr_html_(self):
        return self.get_html_string()

def _row_as_table(df):
    cols = df.columns

    t = DFTable()
    t.field_names = ["Column", "Value"]
    t.align = "r"
    row = df.limit(1).collect()[0].asDict()
    for col in cols:
        t.add_row([ col, row[col] ])

    return t

def _to_table(df, num_rows=100):
    cols = df.columns

    t = DFTable()
    t.field_names = cols
    t.align = "r"
    for row in df.limit(num_rows).collect():
        d = row.asDict()
        t.add_row([ d[col] for col in cols ])

    return t

import re
import sys
from argparse import ArgumentParser
parser = ArgumentParser()
parser.add_argument("--limit", help="Number of lines to return", type=int, default=100)
parser.add_argument("--var", help="Variable name to hold the dataframe", type=str)

@register_line_cell_magic
def sql(line, cell=None):
    """Spark SQL magic
    """
    from pyspark.sql import SparkSession
    spark = SparkSession.builder.appName("Jupyter").getOrCreate()
    if cell is None:
        return _to_table(spark.sql(line))
    elif line:
        df = spark.sql(cell)

        (args, others) = parser.parse_known_args([ arg for arg in re.split("\s+", line) if arg ])

        if args.var:
            setattr(sys.modules[__name__], args.var, df)

        if args.limit == 1:
            return _row_as_table(df)
        else:
            return _to_table(df, num_rows=args.limit)
    else:
        return _to_table(spark.sql(cell))
