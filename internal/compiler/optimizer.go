// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package compiler

import (
	"math"
	"math/big"

	"gopkg.microglot.org/mglotc/internal/idl"
	"gopkg.microglot.org/mglotc/internal/proto"
)

// optimize() applies optimizations to an Image of linked Module descriptors.
func optimize(image *idl.Image) {
	optimizer := imageOptimizer{
		image: image,
	}
	optimizer.optimize()
}

type imageOptimizer struct {
	image *idl.Image
}

// the unfold* family of functions are responsible for coercing proto.Value into a native type
// for purposes of the limited expression evaluation that occurs during constant folding.

func unfoldInteger(value *proto.Value, unfolded *big.Int) bool {
	i := new(big.Int)
	switch valueKind := value.Kind.(type) {
	case *proto.Value_Int8:
		i.SetInt64(int64(valueKind.Int8.Value))
	case *proto.Value_Int16:
		i.SetInt64(int64(valueKind.Int16.Value))
	case *proto.Value_Int32:
		i.SetInt64(int64(valueKind.Int32.Value))
	case *proto.Value_Int64:
		i.SetInt64(valueKind.Int64.Value)
	case *proto.Value_UInt8:
		i.SetUint64(uint64(valueKind.UInt8.Value))
	case *proto.Value_UInt16:
		i.SetUint64(uint64(valueKind.UInt16.Value))
	case *proto.Value_UInt32:
		i.SetUint64(uint64(valueKind.UInt32.Value))
	case *proto.Value_UInt64:
		i.SetUint64(valueKind.UInt64.Value)
	default:
		return false
	}
	*unfolded = *i
	return true
}

func unfoldFloat(value *proto.Value, unfolded *big.Float) bool {
	f := new(big.Float)
	switch valueKind := value.Kind.(type) {
	case *proto.Value_Float32:
		f.SetFloat64(float64(valueKind.Float32.Value))
	case *proto.Value_Float64:
		f.SetFloat64(valueKind.Float64.Value)
	default:
		return false
	}
	*unfolded = *f
	return true
}

func unfoldBoolean(value *proto.Value, unfolded *bool) bool {
	switch valueKind := value.Kind.(type) {
	case *proto.Value_Bool:
		*unfolded = valueKind.Bool.Value
		return true
	}
	return false
}

// the fold* family of functions are all involved in constant folding, and all return *proto.Value.
// they all return 'nil' if no folding occurs, including (since constant folding happens *before* type checking)
// any case where the operand types are unexpected!
// TODO 2023.12.20: this is a partial implementation of expression evaluation, so keep an eye out for
// reuse/refactoring opportunities when building full expression evaluation as part of `impl`

func foldInteger(i *big.Int) *proto.Value {
	if i.IsUint64() {
		i := i.Uint64()
		if i > math.MaxUint32 {
			return &proto.Value{
				Kind: &proto.Value_UInt64{
					UInt64: &proto.ValueUInt64{
						Value: i,
					},
				},
			}
		} else if i > math.MaxUint16 {
			return &proto.Value{
				Kind: &proto.Value_UInt32{
					UInt32: &proto.ValueUInt32{
						Value: uint32(i),
					},
				},
			}
		} else if i > math.MaxUint8 {
			return &proto.Value{
				Kind: &proto.Value_UInt16{
					UInt16: &proto.ValueUInt16{
						Value: uint32(i),
					},
				},
			}
		} else {
			return &proto.Value{
				Kind: &proto.Value_UInt8{
					UInt8: &proto.ValueUInt8{
						Value: uint32(i),
					},
				},
			}
		}
	} else if i.IsInt64() {
		i := i.Int64()
		if (i > math.MaxInt32) || (i < math.MinInt32) {
			return &proto.Value{
				Kind: &proto.Value_Int64{
					Int64: &proto.ValueInt64{
						Value: i,
					},
				},
			}
		} else if (i > math.MaxInt16) || (i < math.MinInt16) {
			return &proto.Value{
				Kind: &proto.Value_Int32{
					Int32: &proto.ValueInt32{
						Value: int32(i),
					},
				},
			}
		} else if (i > math.MaxInt8) || (i < math.MinInt8) {
			return &proto.Value{
				Kind: &proto.Value_Int16{
					Int16: &proto.ValueInt16{
						Value: int32(i),
					},
				},
			}
		} else {
			return &proto.Value{
				Kind: &proto.Value_Int8{
					Int8: &proto.ValueInt8{
						Value: int32(i),
					},
				},
			}
		}
	}
	return nil
}

func foldFloat(f *big.Float) *proto.Value {
	f32, accuracy := f.Float32()
	if !((f32 == 0 && accuracy == big.Below) || (f32 == -0 && accuracy == big.Above) || (f32 == float32(math.Inf(+1)) && accuracy == big.Above) || (f32 == float32(math.Inf(-1)) && accuracy == big.Below)) {
		return &proto.Value{
			Kind: &proto.Value_Float32{
				Float32: &proto.ValueFloat32{
					Value: f32,
				},
			},
		}
	}
	f64, accuracy := f.Float64()
	if !((f64 == 0 && accuracy == big.Below) || (f64 == -0 && accuracy == big.Above) || (f64 == math.Inf(+1) && accuracy == big.Above) || (f64 == math.Inf(-1) && accuracy == big.Below)) {
		return &proto.Value{
			Kind: &proto.Value_Float64{
				Float64: &proto.ValueFloat64{
					Value: f64,
				},
			},
		}
	}
	return nil
}

func foldBoolean(b bool) *proto.Value {
	return &proto.Value{
		Kind: &proto.Value_Bool{
			Bool: &proto.ValueBool{
				Value: b,
			},
		},
	}
}

func foldUnaryPositive(value *proto.Value) *proto.Value {
	var i big.Int
	var f big.Float

	switch {
	case unfoldInteger(value, &i):
		return foldInteger(&i)
	case unfoldFloat(value, &f):
		return foldFloat(&f)
	}
	return nil
}

func foldUnaryNegative(value *proto.Value) *proto.Value {
	var i big.Int
	var f big.Float

	switch {
	case unfoldInteger(value, &i):
		x := new(big.Int)
		return foldInteger(x.Neg(&i))
	case unfoldFloat(value, &f):
		x := new(big.Float)
		return foldFloat(x.Neg(&f))
	}
	return nil
}

func foldUnaryNot(value *proto.Value) *proto.Value {
	var b bool
	switch {
	case unfoldBoolean(value, &b):
		return foldBoolean(!b)
	}
	return nil
}

func foldBinaryOr(left *proto.Value, right *proto.Value) *proto.Value {
	var l, r bool
	switch {
	case unfoldBoolean(left, &l) && unfoldBoolean(right, &r):
		return foldBoolean(l || r)
	}
	return nil
}

func foldBinaryAnd(left *proto.Value, right *proto.Value) *proto.Value {
	var l, r bool
	switch {
	case unfoldBoolean(left, &l) && unfoldBoolean(right, &r):
		return foldBoolean(l && r)
	}
	return nil
}

func foldBinaryEqual(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	var lb, rb bool
	switch {
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) == 0)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) == 0)
	case unfoldBoolean(left, &lb) && unfoldBoolean(right, &rb):
		return foldBoolean(lb == rb)
	}
	return nil
}

func foldBinaryNotEqual(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	var lb, rb bool
	switch {
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) != 0)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) != 0)
	case unfoldBoolean(left, &lb) && unfoldBoolean(right, &rb):
		return foldBoolean(lb != rb)
	}
	return nil
}

func foldBinaryLessThan(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) == -1)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) == -1)
	}
	return nil
}

func foldBinaryLessThanEqual(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) != 1)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) != 1)
	}
	return nil
}

func foldBinaryGreaterThan(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) == 1)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) == 1)
	}
	return nil
}

func foldBinaryGreaterThanEqual(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		return foldBoolean((&li).Cmp(&ri) != -1)
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		return foldBoolean((&lf).Cmp(&rf) != -1)
	}
	return nil
}

func foldBinaryAdd(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		x := new(big.Int)
		return foldInteger(x.Add(&li, &ri))
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		x := new(big.Float)
		return foldFloat(x.Add(&lf, &rf))
	}
	return nil
}

func foldBinarySubtract(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		x := new(big.Int)
		return foldInteger(x.Sub(&li, &ri))
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		x := new(big.Float)
		return foldFloat(x.Sub(&lf, &rf))
	}
	return nil
}

func foldBinaryMultiply(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		x := new(big.Int)
		return foldInteger(x.Mul(&li, &ri))
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		x := new(big.Float)
		return foldFloat(x.Mul(&lf, &rf))
	}
	return nil
}

func foldBinaryDivide(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	var lf, rf big.Float
	switch {
	// TODO 2023.12.20: consider supporting heterogeneous operands?
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		x := new(big.Int)
		return foldInteger(x.Quo(&li, &ri))
	case unfoldFloat(left, &lf) && unfoldFloat(right, &rf):
		x := new(big.Float)
		return foldFloat(x.Quo(&lf, &rf))
	}
	return nil
}

func foldBinaryModulo(left *proto.Value, right *proto.Value) *proto.Value {
	var li, ri big.Int
	switch {
	case unfoldInteger(left, &li) && unfoldInteger(right, &ri):
		x := new(big.Int)
		return foldInteger(x.Mod(&li, &ri))
	}
	return nil
}

func (o *imageOptimizer) optimizeValue(value *proto.Value) {
	fold := func(folded *proto.Value) {
		if folded != nil {
			value.Kind = folded.Kind
		}
	}

	switch valueKind := value.Kind.(type) {
	case *proto.Value_Unary:
		o.optimizeValue(valueKind.Unary.Value)
		switch valueKind.Unary.Operation {
		case proto.OperationUnary_OperationUnaryPositive:
			fold(foldUnaryPositive(valueKind.Unary.Value))
		case proto.OperationUnary_OperationUnaryNegative:
			fold(foldUnaryNegative(valueKind.Unary.Value))
		case proto.OperationUnary_OperationUnaryNot:
			fold(foldUnaryNot(valueKind.Unary.Value))
		}

	case *proto.Value_Binary:
		o.optimizeValue(valueKind.Binary.Left)
		o.optimizeValue(valueKind.Binary.Right)
		switch valueKind.Binary.Operation {
		case proto.OperationBinary_OperationBinaryOr:
			fold(foldBinaryOr(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryAnd:
			fold(foldBinaryAnd(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryEqual:
			fold(foldBinaryEqual(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryNotEqual:
			fold(foldBinaryNotEqual(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryLessThan:
			fold(foldBinaryLessThan(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryLessThanEqual:
			fold(foldBinaryLessThanEqual(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryGreaterThan:
			fold(foldBinaryGreaterThan(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryGreaterThanEqual:
			fold(foldBinaryGreaterThanEqual(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryAdd:
			fold(foldBinaryAdd(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinarySubtract:
			fold(foldBinarySubtract(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryBinOr:
			// TODO 2023.12.20: constant-folding of binor?
		case proto.OperationBinary_OperationBinaryBinAnd:
			// TODO 2023.12.20: constant-folding of binand?
		case proto.OperationBinary_OperationBinaryShiftLeft:
			// TODO 2023.12.20: constant-folding of shiftleft?
		case proto.OperationBinary_OperationBinaryShiftRight:
			// TODO 2023.12.20: constant-folding of shiftright?
		case proto.OperationBinary_OperationBinaryMultiply:
			fold(foldBinaryMultiply(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryDivide:
			fold(foldBinaryDivide(valueKind.Binary.Left, valueKind.Binary.Right))
		case proto.OperationBinary_OperationBinaryModulo:
			fold(foldBinaryModulo(valueKind.Binary.Left, valueKind.Binary.Right))
		}

	case *proto.Value_Identifier:
		switch identifierReference := valueKind.Identifier.Reference.(type) {
		case *proto.ValueIdentifier_Type:
			kind, declaration := o.image.Lookup(identifierReference.Type)
			if kind == idl.TypeKindConstant {
				// constant propagation
				value.Kind = declaration.(*proto.Constant).Value.Kind
			}
		}

	default:
		var i big.Int
		var f big.Float
		switch {
		case unfoldInteger(value, &i):
			fold(foldInteger(&i))
		case unfoldFloat(value, &f):
			fold(foldFloat(&f))
		}
	}
}

func (o *imageOptimizer) optimize() {
	for _, module := range o.image.Modules {
		walkModule(module, func(node interface{}) {
			switch n := node.(type) {
			case *proto.Value:
				o.optimizeValue(n)
			}
		})
	}
}
