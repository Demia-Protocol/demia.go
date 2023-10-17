//go:build ignore

package gen

//go:generate go run github.com/iotaledger/hive.go/codegen/features/cmd@86973b2edb3b slot_identifier.tmpl ../block_id.gen.go BlockID b "ids"
//go:generate go run github.com/iotaledger/hive.go/codegen/features/cmd@86973b2edb3b slot_identifier.tmpl ../commitment_id.gen.go CommitmentID c "ids"
//go:generate go run github.com/iotaledger/hive.go/codegen/features/cmd@86973b2edb3b slot_identifier.tmpl ../transaction_id.gen.go TransactionID t "ids"
//go:generate go run github.com/iotaledger/hive.go/codegen/features/cmd@86973b2edb3b slot_identifier.tmpl ../signed_transaction_id.gen.go SignedTransactionID t "ids"
//go:generate go run github.com/iotaledger/hive.go/codegen/features/cmd@86973b2edb3b slot_identifier.tmpl ../output_id.gen.go OutputID o "ids output"
