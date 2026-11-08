import os
import re

for filename in os.listdir("tests"):
    if not filename.endswith("_test.go") or filename in ["memstore_test.go", "table_test.go", "storetest_test.go", "read_test.go", "filter_test.go", "no_ui_test.go"]: continue
    path = os.path.join("tests", filename)
    with open(path) as f:
        content = f.read()

    for m in ["mockRecord", "secRecord", "serverOnlyRecord", "publicRecord"]:
        if f"func (m *{m}) ModelName() string" in content and f"func (m *{m}) Allow()" not in content:
            content = content.replace(f"func (m *{m}) ModelName() string", f"func (m *{m}) Allow() model.Action {{ return model.AllActions }}\nfunc (m *{m}) ModelName() string")

    # In bench_test.go
    content = re.sub(
        r"reg\.Create\([^,]+,\s*func\(c crud\.Caller, list model\.ModelSlice\) error \{ return nil \}\)",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore()})", content
    )
    content = re.sub(r"reg\.Update\([^\)]+\)", "", content)
    content = re.sub(r"reg\.Delete\([^\)]+\)", "", content)

    # In public_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &mockRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )
    content = content.replace("[]model.ModuleNaming{", "[]crud.Public{")

    # In caller_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &mockRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )

    # In server_only_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &serverOnlyRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &serverOnlyRecord{}, NewList: func() model.ModelSlice { return &serverOnlyRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )
    content = re.sub(
        r"reg\.Update\(func\(\) model\.Model \{ return &serverOnlyRecord\{\} \}, func\(c crud\.Caller, ids, fields \[\]string, rec model\.Model\) error \{",
        r"reg.Table(crud.Table{Model: &serverOnlyRecord{}, NewList: func() model.ModelSlice { return &serverOnlyRecordList{} }, Store: newMemStore(), Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error {", content
    )

    # In security_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &secRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &secRecord{}, NewList: func() model.ModelSlice { return &secRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )
    content = re.sub(
        r"reg\.Update\(func\(\) model\.Model \{ return &secRecord\{\} \}, func\(c crud\.Caller, ids, fields \[\]string, rec model\.Model\) error \{",
        r"reg.Table(crud.Table{Model: &secRecord{}, NewList: func() model.ModelSlice { return &secRecordList{} }, Store: newMemStore(), Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error {", content
    )
    content = re.sub(
        r"reg\.Delete\(&secRecord\{\}, func\(c crud\.Caller, ids \[\]string\) error \{",
        r"reg.Table(crud.Table{Model: &secRecord{}, NewList: func() model.ModelSlice { return &secRecordList{} }, Store: newMemStore(), Delete: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )

    # In mount_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &mockRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )
    content = re.sub(
        r"reg\.Update\(func\(\) model\.Model \{ return &mockRecord\{\} \}, func\(c crud\.Caller, ids, fields \[\]string, rec model\.Model\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error {", content
    )
    content = re.sub(
        r"reg\.Delete\(&mockRecord\{\}, m\.remove\)",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Delete: func(c crud.Caller, tx crud.Tx, rec model.Model) error { return m.remove(c, []string{*rec.Pointers()[0].(*string)}) }})", content
    )
    content = re.sub(
        r"reg\.Delete\(&mockRecord\{\}, func\(c crud\.Caller, ids \[\]string\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Delete: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )

    # In args_json_test.go
    content = re.sub(
        r"reg\.Create\(func\(\) model\.ModelSlice \{ return &mockRecordList\{\} \}, func\(c crud\.Caller, list model\.ModelSlice\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Create: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )
    content = re.sub(
        r"reg\.Update\(func\(\) model\.Model \{ return &mockRecord\{\} \}, func\(c crud\.Caller, ids, fields \[\]string, rec model\.Model\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Update: func(c crud.Caller, tx crud.Tx, old, next model.Model, fields []string) error {", content
    )
    content = re.sub(
        r"reg\.Delete\(&mockRecord\{\}, func\(c crud\.Caller, ids \[\]string\) error \{",
        r"reg.Table(crud.Table{Model: &mockRecord{}, NewList: func() model.ModelSlice { return &mockRecordList{} }, Store: newMemStore(), Delete: func(c crud.Caller, tx crud.Tx, rec model.Model) error {", content
    )

    with open(path, "w") as f:
        f.write(content)
