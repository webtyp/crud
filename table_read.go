package crud

import (
	"strings"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

func readHandler(t Table, schema []model.Field) func(ctx router.Context) {
	return func(ctx router.Context) {
		var filter model.Model
		if t.NewFilter != nil {
			filter = t.NewFilter()
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
			row := t.NewList().Append().(model.Model)
			if err := t.Store.ReadOne(row, args.ID); err != nil {
				router.WriteError(ctx, err)
				return
			}
			list := t.NewList()
			r2 := list.Append().(model.Model)
			if err := t.Store.ReadOne(r2, args.ID); err != nil {
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

		caller := callerOf(ctx)
		var conds []storage.Condition

		if args.Search != "" {
			if t.Search != nil {
				hooks, err := t.Search(caller, args.Search)
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
				for _, f := range schema {
					if f.Item == model.ItemLabel {
						labelField = f.Name
						break
					}
				}
				if labelField == "" {
					router.WriteError(ctx, ErrNoSearchField)
					return
				}
				conds = append(conds, storage.Like(labelField, "%"+strings.ToLower(args.Search)+"%"))
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

		list := t.NewList()

		err := t.Store.ReadAll(
			func() model.Model { return list.Append().(model.Model) },
			q,
			func(m model.Model) {},
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
