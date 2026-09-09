// Restrict upstream namespace imports to parser/serializer primitives. The
// selector engine does not need CSS declaration grammar or MDN property data.
export {default as parse} from 'css-tree/selector-parser';
export {default as generate} from 'css-tree/generator';
export {ident} from 'css-tree/utils';
import walk from 'css-tree/walker';
import convertor from 'css-tree/convertor';
export {walk};
export const find=walk.find;
export const {toPlainObject,fromPlainObject}=convertor;
