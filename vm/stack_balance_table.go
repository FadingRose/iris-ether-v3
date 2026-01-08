package vm

type StackBalanceTable map[OpCode]struct{ paramSize, pushbackSize int }

var stackBalanceTable *StackBalanceTable

func GetStackBalanceTable() *StackBalanceTable {
	if stackBalanceTable != nil {
		return stackBalanceTable
	}

	return &StackBalanceTable{
		STOP:           {0, 0},
		ADD:            {2, 1},
		MUL:            {2, 1},
		SUB:            {2, 1},
		DIV:            {2, 1},
		SDIV:           {2, 1},
		MOD:            {2, 1},
		SMOD:           {2, 1},
		ADDMOD:         {3, 1},
		MULMOD:         {3, 1},
		EXP:            {2, 1},
		SIGNEXTEND:     {2, 1},
		LT:             {2, 1},
		GT:             {2, 1},
		SLT:            {2, 1},
		SGT:            {2, 1},
		EQ:             {2, 1},
		ISZERO:         {1, 1},
		AND:            {2, 1},
		OR:             {2, 1},
		XOR:            {2, 1},
		NOT:            {1, 1},
		BYTE:           {2, 1},
		SHL:            {2, 1},
		SHR:            {2, 1},
		SAR:            {2, 1},
		KECCAK256:      {2, 1},
		ADDRESS:        {0, 1},
		BALANCE:        {1, 1},
		ORIGIN:         {0, 1},
		CALLER:         {0, 1},
		CALLVALUE:      {0, 1},
		CALLDATALOAD:   {1, 1},
		CALLDATASIZE:   {0, 1},
		CALLDATACOPY:   {3, 0},
		CODESIZE:       {0, 1},
		CODECOPY:       {3, 0},
		GASPRICE:       {0, 1},
		EXTCODESIZE:    {1, 1},
		EXTCODECOPY:    {4, 0},
		RETURNDATASIZE: {0, 1},
		RETURNDATACOPY: {3, 0},
		EXTCODEHASH:    {1, 1},
		BLOCKHASH:      {1, 1},
		COINBASE:       {0, 1},
		TIMESTAMP:      {0, 1},
		NUMBER:         {0, 1},
		DIFFICULTY:     {0, 1},
		GASLIMIT:       {0, 1},
		CHAINID:        {0, 1},
		SELFBALANCE:    {0, 1},
		BASEFEE:        {0, 1},
		BLOBHASH:       {1, 1},
		BLOBBASEFEE:    {0, 1},
		POP:            {1, 0},
		MLOAD:          {1, 1},
		MSTORE:         {2, 0},
		MSTORE8:        {2, 0},
		SLOAD:          {1, 1},
		SSTORE:         {2, 0},
		JUMP:           {1, 0},
		JUMPI:          {2, 0},
		PC:             {0, 1},
		MSIZE:          {0, 1},
		GAS:            {0, 1},
		JUMPDEST:       {0, 0},
		TLOAD:          {1, 1},
		TSTORE:         {2, 0},
		MCOPY:          {3, 0},
		LOG0:           {2, 0},
		LOG1:           {3, 0},
		LOG2:           {4, 0},
		LOG3:           {5, 0},
		LOG4:           {6, 0},
		CREATE:         {3, 1},
		CALL:           {7, 1},
		CALLCODE:       {7, 1},
		RETURN:         {2, 0},
		DELEGATECALL:   {6, 1},
		CREATE2:        {4, 1},
		STATICCALL:     {6, 1},
		REVERT:         {2, 0},
		SELFDESTRUCT:   {1, 0},
		INVALID:        {0, 0},
	}
}

func GetStackSize(op OpCode) (paramSize, pushbackSize int) {
	if stackBalanceTable == nil {
		stackBalanceTable = GetStackBalanceTable()
	}
	if op.IsPush() {
		return 0, 1
	}
	if op.IsDup() {
		// return op.DupNum(), 1
		return 0, 1
	}
	if op.IsSwap() {
		return 0, 0
	}

	return (*stackBalanceTable)[op].paramSize, (*stackBalanceTable)[op].pushbackSize
}
