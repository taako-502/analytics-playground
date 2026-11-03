/* 実際のマートを描画する。比較の基準はページを開いている間だけ保持する。 */
let reportData = null;
let reportBaseline = null;
const yen = n => '¥' + Math.round(n).toLocaleString('ja-JP');
const escapeXML = s => String(s).replace(/[<>&"']/g, c => ({'<':'&lt;','>':'&gt;','&':'&amp;','"':'&quot;',"'":'&apos;'}[c]));
const totals = rows => rows.reduce((a,r) => ({gross:a.gross+Number(r.gross_sales),refund:a.refund+Number(r.returns),net:a.net+Number(r.net_sales)}),{gross:0,refund:0,net:0});
function renderReport(data) {
  reportData = data;
  const root = document.querySelector('#report-content');
  const failed = ['chartDaily','chartProducts','chartMonthly'].some(k => data[k+'Error']);
  if(failed || !data.chartDaily?.length) {
    root.replaceChildren();
    const p=document.createElement('p');p.className='report-empty';
    p.textContent=failed?'グラフ用データを取得できません。接続と各レイヤーのエラーを確認してください。':'まだ集計データがありません。「サンプルデータを追加」→「すべて実行」でグラフを表示できます。';
    root.append(p);return;
  }
  const daily=data.chartDaily, products=data.chartProducts||[], monthly=data.chartMonthly||[];
  const sum=totals(daily), latest=monthly.at(-1);
  const ink='#172d40', muted='#62788b', blue='#2767d8', teal='#008b80', orange='#e48a35';
  let parts=[`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 1040" role="img" aria-labelledby="report-title report-desc"><title id="report-title">EC売上分析</title><desc id="report-desc">架空のECのマート全期間を表示。日別売上、商品別売上上位10件、月別リピート率。</desc><rect width="1200" height="1040" fill="#f5f8fc"/>`];
  const rect=(x,y,w,h,fill='#fff')=>parts.push(`<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="12" fill="${fill}"/>`);
  const text=(x,y,s,size=14,color=ink,weight=400,anchor='start')=>parts.push(`<text x="${x}" y="${y}" font-family="system-ui, sans-serif" font-size="${size}" fill="${color}" font-weight="${weight}" text-anchor="${anchor}">${escapeXML(s)}</text>`);
  const line=(x,y,x2,y2,color='#e4ebf2')=>parts.push(`<line x1="${x}" y1="${y}" x2="${x2}" y2="${y2}" stroke="${color}"/>`);
  text(32,38,'ANALYTICS PLAYGROUND / 架空EC',13,teal,700);
  text(32,79,'データの流れから、売上を読む。',30,ink,700);
  text(32,107,`${daily[0].day.slice(0,10)} — ${daily.at(-1).day.slice(0,10)}  /  マート全期間・日本時間`,14,muted);
  const steps=[['業務DB','注文・返品'],['データレイク','JSONを保存'],['raw','変換前データ'],['DWH','分析用の明細'],['データマート','指標を集計'],['BI','可視化']];
  steps.forEach(([a,b],i)=>{let x=32+i*192;rect(x,128,176,65,i===4?'#e0f3ef':'#eaf0f8');text(x+12,152,a,15,ink,700);text(x+12,175,b,12,muted);if(i<5)text(x+179,166,'›',19,muted);});
  const kpis=[['売上（返品控除前）',yen(sum.gross)],['返品額',yen(sum.refund)],['純売上',yen(sum.net)],['最新月のリピート率',latest?`${(Number(latest.repeat_rate)*100).toFixed(1)}%`:'—']];
  kpis.forEach(([a,b],i)=>{let x=32+i*288;rect(x,211,272,98);text(x+18,239,a,13,muted);text(x+18,276,b,29,i===2?teal:ink,700);if(i===3&&latest)text(x+18,297,latest.month.slice(0,7),11,muted);});
  rect(32,327,1136,300);text(52,357,'日別売上と返品',18,ink,700);
  [['売上',blue],['純売上',teal],['返品',orange]].forEach(([label,c],i)=>{rect(770+i*124,345,12,12,c);text(790+i*124,356,label,13,muted);});
  const min=Math.min(0,...daily.map(r=>Number(r.net_sales))), max=Math.max(1,...daily.map(r=>Number(r.gross_sales)),...daily.map(r=>Number(r.returns)));
  const firstDay=Date.parse(daily[0].day), lastDay=Date.parse(daily.at(-1).day);
  const x=i=>112+(Date.parse(daily[i].day)-firstDay)*1020/Math.max(1,lastDay-firstDay), y=v=>575-(v-min)/(max-min)*188;
  for(let i=0;i<=4;i++){let v=min+(max-min)*i/4;line(112,y(v),1132,y(v));text(99,y(v)+4,Math.round(v).toLocaleString('ja-JP'),11,muted,400,'end');}
  text(52,382,'円',11,muted);
  for(const [key,c] of [['gross_sales',blue],['net_sales',teal],['returns',orange]]) {
    parts.push(`<polyline fill="none" stroke="${c}" stroke-width="2.5" stroke-linejoin="round" points="${daily.map((r,i)=>`${x(i)},${y(Number(r[key]))}`).join(' ')}"/>`);
    if(daily.length<12)daily.forEach((r,i)=>parts.push(`<circle cx="${x(i)}" cy="${y(Number(r[key]))}" r="3.5" fill="${c}"/>`));
  }
  [...new Set([0,Math.floor((daily.length-1)/2),daily.length-1])].forEach(i=>text(x(i),600,daily[i].day.slice(0,10),11,muted,400,'middle'));
  rect(32,645,558,282);text(52,675,'商品別売上 TOP 10',18,ink,700);text(52,698,'全期間の合計・返品控除前',12,muted);
  const best=Math.max(1,...products.map(r=>Number(r.sales)));
  products.forEach((r,i)=>{const yy=721+i*19; text(155,yy+10,r.product_name.length>10?r.product_name.slice(0,9)+'…':r.product_name,11,muted,400,'end');rect(167,yy,Math.max(1,Number(r.sales)/best*275),12,teal);text(570,yy+10,yen(Number(r.sales)),11,ink,400,'end');});
  rect(610,645,558,282);text(630,675,'月別リピート率',18,ink,700);text(630,698,'月内に2回以上購入した顧客 ÷ 月内の購入顧客',12,muted);
  const mx=i=>686+i*438/Math.max(1,monthly.length-1), my=v=>867-v*140;
  [0,.5,1].forEach(v=>{line(686,my(v),1124,my(v));text(676,my(v)+4,`${v*100}%`,11,muted,400,'end');});
  parts.push(`<polyline fill="none" stroke="${blue}" stroke-width="3" points="${monthly.map((r,i)=>`${mx(i)},${my(Number(r.repeat_rate))}`).join(' ')}"/>`);
  monthly.forEach((r,i)=>{parts.push(`<circle cx="${mx(i)}" cy="${my(Number(r.repeat_rate))}" r="4" fill="${blue}"/>`);if(monthly.length<=6){text(mx(i),my(Number(r.repeat_rate))-10,`${(Number(r.repeat_rate)*100).toFixed(1)}%`,11,blue,700,'middle');text(mx(i),892,r.month.slice(0,7),11,muted,400,'middle');}});
  if(monthly.length>6)[0,monthly.length-1].forEach(i=>text(mx(i),892,monthly[i].month.slice(0,7),11,muted,400,'middle'));
  if(reportBaseline){const before=totals(reportBaseline.rows);text(32,956,`基準保存時 → 現在  |  返品額 ${yen(before.refund)} → ${yen(sum.refund)}  |  純売上 ${yen(before.net)} → ${yen(sum.net)}`,15,ink,700);text(32,980,`比較基準: ${reportBaseline.at}（このページを開いている間のみ保持）`,12,muted);}
  else{text(32,956,'売上は決済日、返品は返品完了日に計上。各段階を実行すると後続に反映されます。',13,muted);text(32,980,'「比較の基準を保存」→ 返品を追加 → 各段階を実行すると、ここに変更前後の金額を表示します。',12,muted);}
  text(32,1014,`抽出バッチ: ${data.batch||'なし'}`,11,muted);
  parts.push('</svg>');root.innerHTML=parts.join('');
}
document.querySelector('#presentation').onclick=()=>{const active=document.body.classList.toggle('presentation');document.querySelector('#presentation').textContent=active?'操作画面に戻る':'記事用表示';};
document.querySelector('#baseline').onclick=()=>{if(!reportData?.chartDaily?.length||reportData.chartDailyError)return;reportBaseline={rows:structuredClone(reportData.chartDaily),at:new Date().toLocaleString('ja-JP',{timeZone:'Asia/Tokyo'})+' JST'};renderReport(reportData);};
document.querySelector('#save-image').onclick=async()=>{
  const status=document.querySelector('#export-status'); const svg=document.querySelector('#report svg');
  if(!svg){status.textContent='集計データを表示してから保存してください。';return;}
  let url;
  try {
    await document.fonts.ready;
    url=URL.createObjectURL(new Blob([new XMLSerializer().serializeToString(svg)],{type:'image/svg+xml;charset=utf-8'}));
    const img=new Image();img.src=url;await img.decode();
    const canvas=document.createElement('canvas');canvas.width=2400;canvas.height=2080;canvas.getContext('2d').drawImage(img,0,0,2400,2080);
    const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/png'));if(!blob)throw Error('画像変換に失敗しました');
    const png=URL.createObjectURL(blob), a=document.createElement('a');a.href=png;a.download='analytics-report.png';a.click();setTimeout(()=>URL.revokeObjectURL(png),1000);status.textContent='PNGを保存しました（2400 × 2080）。';
  }catch(e){status.textContent='保存できませんでした: '+e.message;}finally{if(url)URL.revokeObjectURL(url);}
};
