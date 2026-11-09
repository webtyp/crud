package crud

import (
	"webtyp.com/fmt"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

func readRecordsHandler(entity Table, schemaFields []model.Field) func(ctx router.Context) {
	return func(ctx router.Context) {
		var filter model.Model
		if entity.NewFilter != nil {
			filter = entity.NewFilter()
		}

		args := ReadArgs{}
		if filter != nil {
			args.Filter = filter
		}
		if err := ctx.Decode(&args); err != nil {
			router.WriteError(ctx, ErrCannotRead)
			return
		}
		if err := validateReadArgs(&args); err != nil {
			router.WriteError(ctx, err)
			return
		}

		if args.ID != "" {
			row := entity.NewList().Append().(model.Model)
			if err := entity.Store.ReadOne(row, args.ID); err != nil {
				router.WriteError(ctx, err)
				return
			}
			list := entity.NewList()
			r2 := list.Append().(model.Model)
			if err := entity.Store.ReadOne(r2, args.ID); err != nil {
				router.WriteError(ctx, err)
				return
			}
			if enc, ok := list.(model.Encodable); ok {
				if err := ctx.Encode(enc); err != nil {
					router.WriteError(ctx, err)
				}
			}
			return
		}

		caller := currentUserOf(ctx)
		var conds []storage.Condition

		if args.Search != "" {
			if entity.Search != nil {
				hooks, err := entity.Search(caller, args.Search)
				if err != nil {
					router.WriteError(ctx, err)
					return
				}
				for _, c := range hooks {
					if c.Logic() == "OR" {
						router.WriteError(ctx, ErrOrCondition)
						return
					}
				}
				conds = append(conds, hooks...)
			} else {
				var labelField string
				for _, f := range schemaFields {
					if f.Item == model.ItemLabel {
						labelField = f.Name
						break
					}
				}
				if labelField == "" {
					router.WriteError(ctx, ErrNoSearchField)
					return
				}
				conds = append(conds, storage.Like(labelField, "%"+fmt.ToLower(args.Search)+"%"))
			}
		}

		if filter != nil {
			ptrs := filter.Pointers()
			fschema := filter.Schema()
			for i, f := range fschema {
				switch v := ptrs[i].(type) {
				case *string:
					if *v != "" {
						conds = append(conds, storage.Eq(f.Name, *v))
					}
				case *int64:
					if *v != 0 {
						conds = append(conds, storage.Eq(f.Name, *v))
					}
				case *bool:
					if *v {
						conds = append(conds, storage.Eq(f.Name, *v))
					}
				}
			}
		}

		limit := Cap(args.Limit)
		q := Query{
			Conditions: conds,
			After:      args.After,
			Limit:      limit,
		}

		list := entity.NewList()

		err := entity.Store.ReadAll(
			func() model.Model { return entity.NewList().Append().(model.Model) },
			q,
			func(m model.Model) {
				row := list.Append().(model.Model)
				copyRecord(m, row)
			},
		)

		if err != nil {
			router.WriteError(ctx, err)
			return
		}

		if enc, ok := list.(model.Encodable); ok {
			if err := ctx.Encode(enc); err != nil {
				router.WriteError(ctx, err)
			}
		}
	}
}

func copyRecord(src, dst model.Model) {
	srcPtrs := src.Pointers()
	dstPtrs := dst.Pointers()
	for i := range srcPtrs {
		switch s := srcPtrs[i].(type) {
		case *string:
			*dstPtrs[i].(*string) = *s
		case *int64:
			*dstPtrs[i].(*int64) = *s
		case *int:
			*dstPtrs[i].(*int) = *s
		case *bool:
			*dstPtrs[i].(*bool) = *s
		case *float64:
			*dstPtrs[i].(*float64) = *s
		case *float32:
			*dstPtrs[i].(*float32) = *s
		case *[]string:
			*dstPtrs[i].(*[]string) = *s
		case *[]int:
			*dstPtrs[i].(*[]int) = *s
		case *[]int64:
			*dstPtrs[i].(*[]int64) = *s
		case *[]float64:
			*dstPtrs[i].(*[]float64) = *s
		case *[]byte:
			*dstPtrs[i].(*[]byte) = *s
		}
	}
}
