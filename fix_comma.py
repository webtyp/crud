import re

for filename in ["tests/args_json_test.go", "tests/caller_test.go", "tests/security_test.go", "tests/server_only_test.go", "tests/mount_test.go"]:
    with open(filename) as f:
        c = f.read()
    c = c.replace("*list.(*mockRecordList)", "*rec.(*mockRecord)")
    c = c.replace("*list.(*secRecordList)", "*rec.(*secRecord)")
    c = c.replace("l := mockRecordList{*rec.(*mockRecord)}", "l := mockRecordList{*rec.(*mockRecord)}")
    c = re.sub(r"return m.create\(c, (\*rec\.\(\*mockRecord\))\)", r"l := mockRecordList{\1}\n\t\t\treturn m.create(c, &l)", c)
    # The previous regex left missing commas between Store: newMemStore() and Create: func()
    c = c.replace("Store: newMemStore()", "Store: newMemStore(),")
    c = c.replace("Store: newMemStore(),,", "Store: newMemStore(),")
    with open(filename, "w") as f:
        f.write(c)
