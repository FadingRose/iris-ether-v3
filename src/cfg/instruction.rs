//! # CFG Instruction Representation
//!
//! This module defines the instruction structure used in CFG analysis,
//! including iterators and stack size information.

use crate::vm::{opcode::OpCode, stack_balance_table::get_stack_size};
use std::fmt;

/// A CFG instruction with additional metadata
#[derive(Debug, Clone)]
pub struct Instruction {
    /// Program counter
    pub pc: u64,
    /// Instruction argument (for PUSH* instructions)
    pub arg: Option<Vec<u8>>,
    /// Operation code
    pub op: OpCode,
    /// Offset in bytecode
    pub offset: u64,
    /// Number of parameters consumed from stack
    pub param_size: usize,
    /// Number of values pushed to stack
    pub pushback_size: usize,
}

impl Instruction {
    /// Create a new instruction
    pub fn new(pc: u64, op: OpCode, arg: Option<Vec<u8>>) -> Self {
        let (param_size, pushback_size) = get_stack_size(op);

        Instruction {
            pc,
            arg,
            op,
            offset: 0,
            param_size,
            pushback_size,
        }
    }

    /// Get the program counter
    pub fn pc(&self) -> u64 {
        self.pc
    }

    /// Get the instruction argument as bytes
    pub fn arg_bytes(&self) -> Option<&[u8]> {
        self.arg.as_deref()
    }

    /// Check if instruction is a terminating instruction
    pub fn is_terminal(&self) -> bool {
        self.op.is_halt() || matches!(self.op, OpCode::JUMP | OpCode::JUMPI)
    }

    /// Get the offset in bytecode
    pub fn offset(&self) -> u64 {
        self.offset
    }

    /// Set the offset
    pub fn set_offset(&mut self, offset: u64) {
        self.offset = offset;
    }

    /// Get stack size changes
    pub fn stack_size(&self) -> (usize, usize) {
        (self.param_size, self.pushback_size)
    }
}

impl fmt::Display for Instruction {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let arg_str = if let Some(ref arg) = self.arg {
            if self.op == OpCode::PUSH0 || arg.is_empty() {
                String::new()
            } else {
                format!("0x{}", hex::encode(arg))
            }
        } else {
            String::new()
        };

        write!(f, "0x{:x}({}) {} {}", self.pc, self.pc, self.op, arg_str)
    }
}

/// Iterator over instructions
#[derive(Debug, Clone)]
pub struct InstructionIterator {
    instructions: Vec<Instruction>,
    index: i32,
    error: Option<String>,
}

impl InstructionIterator {
    /// Create a new forward iterator
    pub fn new(instructions: Vec<Instruction>) -> Self {
        InstructionIterator {
            instructions,
            index: -1,
            error: None,
        }
    }

    /// Create a new reverse iterator
    pub fn new_reverse(instructions: Vec<Instruction>) -> Self {
        InstructionIterator {
            instructions,
            index: instructions.len() as i32,
            error: None,
        }
    }

    /// Move to next instruction
    pub fn next(&mut self) -> bool {
        if self.index >= self.instructions.len() as i32 {
            return false;
        }
        if self.error.is_some() {
            return false;
        }
        self.index += 1;
        true
    }

    /// Move to previous instruction
    pub fn prev(&mut self) -> bool {
        if self.index < 0 {
            return false;
        }
        if self.error.is_some() {
            return false;
        }
        self.index -= 1;
        true
    }

    /// Get current instruction values
    pub fn values(&self) -> Option<(u64, Option<Vec<u8>>, OpCode)> {
        if self.index >= self.instructions.len() as i32 {
            self.error = Some("iterator broken".to_string());
            return None;
        }
        if self.index < 0 {
            self.error = Some("iterator not started, call Next() first".to_string());
            return None;
        }

        let inst = &self.instructions[self.index as usize];
        Some((inst.pc, inst.arg.clone(), inst.op))
    }

    /// Get current opcode
    pub fn opcode(&self) -> Option<OpCode> {
        if self.index >= self.instructions.len() as i32 {
            self.error = Some("iterator broken".to_string());
            return None;
        }
        if self.index < 0 {
            self.error = Some("iterator not started, call Next() first".to_string());
            return None;
        }

        Some(self.instructions[self.index as usize].op)
    }

    /// Get stack size changes for current instruction
    pub fn stack_size(&self) -> (usize, usize) {
        if self.index >= self.instructions.len() as i32 {
            return (0, 0);
        }
        if self.index < 0 {
            return (0, 0);
        }

        if let Some(op) = self.opcode() {
            get_stack_size(op)
        } else {
            (0, 0)
        }
    }

    /// Get any error
    pub fn error(&self) -> Option<&str> {
        self.error.as_deref()
    }

    /// Get current index
    pub fn index(&self) -> i32 {
        self.index
    }

    /// Reset iterator to beginning
    pub fn reset(&mut self) {
        self.index = -1;
        self.error = None;
    }
}

/// Collection of instructions with iterators
#[derive(Debug, Clone)]
pub struct Instructions {
    inner: Vec<Instruction>,
}

impl Instructions {
    /// Create a new instruction collection
    pub fn new() -> Self {
        Instructions { inner: Vec::new() }
    }

    /// Add an instruction
    pub fn push(&mut self, inst: Instruction) {
        self.inner.push(inst);
    }

    /// Get number of instructions
    pub fn len(&self) -> usize {
        self.inner.len()
    }

    /// Check if empty
    pub fn is_empty(&self) -> bool {
        self.inner.is_empty()
    }

    /// Get instruction by index
    pub fn get(&self, index: usize) -> Option<&Instruction> {
        self.inner.get(index)
    }

    /// Create a forward iterator
    pub fn iter(&self) -> InstructionIterator {
        InstructionIterator::new(self.inner.clone())
    }

    /// Create a reverse iterator
    pub fn iter_reverse(&self) -> InstructionIterator {
        InstructionIterator::new_reverse(self.inner.clone())
    }

    /// Convert to vector
    pub fn to_vec(&self) -> Vec<Instruction> {
        self.inner.clone()
    }
}

impl Default for Instructions {
    fn default() -> Self {
        Self::new()
    }
}

impl From<Vec<Instruction>> for Instructions {
    fn from(vec: Vec<Instruction>) -> Self {
        Instructions { inner: vec }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_instruction_creation() {
        let inst = Instruction::new(0, OpCode::PUSH1, Some(vec![0x01]));
        assert_eq!(inst.pc, 0);
        assert_eq!(inst.op, OpCode::PUSH1);
        assert_eq!(inst.arg, Some(vec![0x01]));
        assert_eq!(inst.param_size, 0);
        assert_eq!(inst.pushback_size, 1);
    }

    #[test]
    fn test_instruction_iterator() {
        let insts = vec![
            Instruction::new(0, OpCode::PUSH1, Some(vec![0x01])),
            Instruction::new(2, OpCode::PUSH1, Some(vec![0x02])),
            Instruction::new(4, OpCode::ADD, None),
        ];

        let mut iter = InstructionIterator::new(insts);
        assert!(iter.next());
        let (pc, arg, op) = iter.values().unwrap();
        assert_eq!(pc, 0);
        assert_eq!(op, OpCode::PUSH1);
        assert_eq!(arg, Some(vec![0x01]));

        assert!(iter.next());
        assert_eq!(iter.opcode(), Some(OpCode::PUSH1));

        assert!(iter.next());
        assert_eq!(iter.opcode(), Some(OpCode::ADD));

        assert!(!iter.next());
    }

    #[test]
    fn test_instruction_stack_size() {
        let inst = Instruction::new(0, OpCode::ADD, None);
        let (param, pushback) = inst.stack_size();
        assert_eq!(param, 2);
        assert_eq!(pushback, 1);
    }
}
