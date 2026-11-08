import os
import re

for filename in os.listdir("tests"):
    if not filename.endswith("_test.go") or filename in ["memstore_test.go", "table_test.go", "storetest_test.go", "read_test.go", "filter_test.go", "no_ui_test.go"]: continue
    path = os.path.join("tests", filename)
    with open(path) as f:
        content = f.read()

    # Add PK to first field in Schema
    content = content.replace("Name: \"id\", Type: model.Text()", "Name: \"id\", Type: model.Text(), DB: &model.FieldDB{PK: true, AutoInc: true}")
    content = content.replace("Name: \"id\", Type: model.Int()", "Name: \"id\", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}")

    with open(path, "w") as f:
        f.write(content)
