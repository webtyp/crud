package crud

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// statusNoContent answers an update or delete: the client decodes nothing.
const statusNoContent = 204

// Op builds the canonical bare operation name for a model and one of the four model.Action verbs
// ("<ModelName>_<action>", e.g. "room_floor_c", "room_floor_r", "room_floor_u", "room_floor_d").
// It uses "_" rather than "." so the qualified operation name "<Module>.<Op>" contains a single dot
// as required by rpc.Caller ("<module>.<op>").
func Op(m model.ModuleNaming, action model.Action) string {
	switch action {
	case model.Create:
		return m.ModelName() + "_c"
	case model.Read:
		return m.ModelName() + "_r"
	case model.Update:
		return m.ModelName() + "_u"
	case model.Delete:
		return m.ModelName() + "_d"
	default:
		return m.ModelName() + "_" + action.String()
	}
}

// MountCreate registers Op(sample, model.Create) as the batch creation operation of an entity:
// it decodes all records sent in {"records":[…]} into a fresh ModelSlice from newList, invokes
// create(list) once so the module can persist the entire batch in a single transaction, and
// answers the last record in the batch (create may fill its id).
// No records → 400; a create error → its own status.
//
//	crud.MountCreate(reg, func() model.ModelSlice { return &DeviceList{} }, func(list model.ModelSlice) error {
//		return m.CreateDevices(*list.(*DeviceList))
//	})
func MountCreate(reg router.OperationRegistry, newList func() model.ModelSlice, create func(list model.ModelSlice) error) router.Route {
	sample := newList().Append().(model.Model)
	res := model.Resource(sample.ModelName())
	return reg.Operation(Op(sample, model.Create), func(ctx router.Context) {
		in := createIn{list: newList()}
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if in.list.Len() == 0 {
			rpc.WriteError(ctx, ErrNoRecords)
			return
		}
		if err := create(in.list); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		last := in.list.At(in.list.Len() - 1).(model.Encodable)
		if err := ctx.Encode(last); err != nil {
			rpc.WriteError(ctx, err)
		}
	}).Requires(res, model.Create).Accepts(sample)
}

// MountRead registers Op(m, model.Read) as the read operation of an entity:
// it decodes ReadArgs, caps args.Limit to MaxPage via Cap, calls read(args) and encodes the
// returned ModelSlice.
//
//	crud.MountRead(reg, &Device{}, func(args crud.ReadArgs) (model.ModelSlice, error) {
//		return m.ReadDevices(args.ID, args.Search, args.After, args.Limit)
//	})
func MountRead(reg router.OperationRegistry, m model.ModuleNaming, read func(args ReadArgs) (model.ModelSlice, error)) router.Route {
	res := model.Resource(m.ModelName())
	return reg.Operation(Op(m, model.Read), func(ctx router.Context) {
		var args ReadArgs
		if err := ctx.Decode(&args); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		args.Limit = Cap(args.Limit)
		list, err := read(args)
		if err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		if enc, ok := list.(model.Encodable); ok {
			if err := ctx.Encode(enc); err != nil {
				rpc.WriteError(ctx, err)
			}
		}
	}).Requires(res, model.Read).Accepts(&ReadArgs{})
}

// MountUpdate registers Op(sample, model.Update) as the field-patch operation of an entity:
// it reads the ids, field names and carrier record sent in {"ids":[…],"fields":[…],"record":{…}}
// and calls update(ids, fields, rec) once. No ids or no fields → 400.
//
//	crud.MountUpdate(reg, func() model.Model { return &Device{} }, func(ids, fields []string, rec model.Model) error {
//		return m.UpdateDevices(ids, fields, *rec.(*Device))
//	})
func MountUpdate(reg router.OperationRegistry, newRecord func() model.Model, update func(ids, fields []string, rec model.Model) error) router.Route {
	sample := newRecord()
	res := model.Resource(sample.ModelName())
	return reg.Operation(Op(sample, model.Update), func(ctx router.Context) {
		in := UpdateArgs{Record: newRecord()}
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if len(in.IDs) == 0 {
			rpc.WriteError(ctx, ErrNoIDs)
			return
		}
		if len(in.Fields) == 0 {
			rpc.WriteError(ctx, ErrNoFields)
			return
		}
		if err := update(in.IDs, in.Fields, in.Record); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(statusNoContent)
	}).Requires(res, model.Update).Accepts(sample)
}

// MountDelete registers Op(m, model.Delete) as the delete operation of an entity:
// it reads the ids sent in {"ids":[…]} and calls remove(ids) once. No ids → 400.
//
//	crud.MountDelete(reg, &Device{}, m.DeleteDevices)
func MountDelete(reg router.OperationRegistry, m model.ModuleNaming, remove func(ids []string) error) router.Route {
	res := model.Resource(m.ModelName())
	return reg.Operation(Op(m, model.Delete), func(ctx router.Context) {
		var in DeleteArgs
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if len(in.IDs) == 0 {
			rpc.WriteError(ctx, ErrNoIDs)
			return
		}
		if err := remove(in.IDs); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(statusNoContent)
	}).Requires(res, model.Delete)
}
