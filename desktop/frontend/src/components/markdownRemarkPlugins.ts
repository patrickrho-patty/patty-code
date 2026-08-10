import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { remarkMathPolicy } from "./remarkMathPolicy";
import { remarkLocalPathLinks } from "../lib/localPathLinks";

// One shared parser policy keeps live Markdown and session exports identical.
export const pattyRemarkPlugins = [remarkGfm, remarkMath, remarkMathPolicy, remarkLocalPathLinks];
export { pattyRehypePlugins } from "./rehypePattyKatex";
