const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
function harness(){
  const elements=new Map();
  const document={querySelector(key){if(!elements.has(key))elements.set(key,{innerHTML:'',replaceChildren(){this.innerHTML='';},append(p){this.innerHTML=p.textContent;}});return elements.get(key);},createElement(){return {};}};
  const context=vm.createContext({document,structuredClone});
  vm.runInContext(fs.readFileSync('web/report.js','utf8'),context);
  return {context,draw(data){context.renderReport(data);return document.querySelector('#report-content').innerHTML;},document};
}
const data={batch:'test',chartDaily:[{day:'2026-10-01',gross_sales:1000,returns:0,net_sales:1000},{day:'2026-10-03',gross_sales:0,returns:1200,net_sales:-1200}],chartProducts:[{product_name:'<script>&商品',sales:1000}],chartMonthly:[{month:'2026-10-01',repeat_rate:0.25}]};
test('negative net sales, percentages, XML escaping and single points',()=>{
 const h=harness();let svg=h.draw(data);assert.match(svg,/¥-200/);assert.match(svg,/25.0%/);assert.match(svg,/&lt;script&gt;&amp;/);assert.doesNotMatch(svg,/NaN|Infinity|<script>/);
 svg=h.draw({...data,chartDaily:[data.chartDaily[0]]});assert.doesNotMatch(svg,/NaN|Infinity/);
});
test('baseline remains fixed after refresh; failed data replaces old graph',()=>{
 const h=harness();h.draw(data);h.document.querySelector('#baseline').onclick();
 const svg=h.draw({...data,chartDaily:[data.chartDaily[0]]});assert.match(svg,/返品額 ¥1,200 → ¥0/);
 assert.doesNotMatch(h.draw({...data,chartDailyError:'offline'}),/<svg/);
 assert.match(h.draw({}),/まだ集計データがありません/);
});
module.exports={harness};
