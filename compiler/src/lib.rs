use uxnsmal::{compiler, lexer, parser, problem, typechecker, bytecode};

unsafe extern "C" {
	fn console_log(ptr: *const u8, len: usize);
	fn load(ptr: *const u8, len: usize);
}
fn log(s: &str) {
	unsafe { console_log(s.as_ptr(), s.len()) };
}

#[unsafe(no_mangle)]
pub extern "C" fn alloc(size: usize) -> *mut u8 {
	let mut buf = Vec::with_capacity(size);
	let ptr = buf.as_mut_ptr();
	core::mem::forget(buf);
	ptr
}

#[unsafe(no_mangle)]
pub extern "C" fn compile_source(ptr: *const u8, len: usize) {
	let bytes = unsafe { core::slice::from_raw_parts(ptr, len) };
	let Ok(source) = core::str::from_utf8(bytes) else {
		panic!("source code is not a valid UTF-8 string");
	};

	set_hook();

	let mut problems = problem::Problems::default();

	match compile(source, &mut problems) {
		Ok(bytecode) => {
			log(&format!("OK {}",bytecode.opcodes.len()));
			if let Some(p) = problems.list.first() {
				log(&p.msg);
			}
			unsafe { load(bytecode.opcodes.as_ptr(), bytecode.opcodes.len()); }
		},
		Err(problem::FatalError) => {
			log(&problems.list[0].msg)
		}
	}
}

fn compile(source: &str, problems: &mut problem::Problems) -> Result<bytecode::Bytecode, problem::FatalError> {
	let tokens = lexer::Lexer::lex(source, problems)?;
	let mut ast = parser::Parser::parse(source, problems, &tokens)?;
	let program = typechecker::Typechecker::check(&mut ast, problems)?;
	let bytecode = compiler::Compiler::compile(&program);

	Ok(bytecode)
}

fn set_hook() {
	std::panic::set_hook(Box::new(|info| {
		if let Some(loc) = info.location() {
			let loc_str = format!(
				"in file {:?} at {}:{}",
				loc.file(),
				loc.line(),
				loc.column()
			);

			log(&loc_str);
		} else {
			log("NO LOCATION");
		}

		let payload: &str;
		if let Some(s) = info.payload().downcast_ref::<&str>() {
			payload = s;
		} else if let Some(s) = info.payload().downcast_ref::<String>() {
			payload = &s;
		} else {
			payload = "NO PAYLOAD";
		}

		log(payload);
	}));
}
