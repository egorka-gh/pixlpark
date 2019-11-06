package com.photodispatcher.factory{
	import com.photodispatcher.context.Context;
	import com.photodispatcher.model.mysql.entities.AttrJsonMap;
	import com.photodispatcher.model.mysql.entities.Order;
	import com.photodispatcher.model.mysql.entities.OrderExtraInfo;
	import com.photodispatcher.model.mysql.entities.OrderState;
	import com.photodispatcher.model.mysql.entities.OrderTemp;
	import com.photodispatcher.model.mysql.entities.Source;
	import com.photodispatcher.model.mysql.entities.SourceType;
	import com.photodispatcher.model.mysql.entities.StateLog;
	import com.photodispatcher.model.mysql.entities.SubOrder;
	import com.photodispatcher.provider.fbook.FBookProject;
	import com.photodispatcher.provider.preprocess.CaptionSetter;
	import com.photodispatcher.util.JsonUtil;
	import com.photodispatcher.util.StrUtil;
	
	import flash.filesystem.File;
	import flash.filesystem.FileMode;
	import flash.filesystem.FileStream;
	import flash.utils.Dictionary;
	
	import mx.collections.ArrayCollection;
	
	public class OrderBuilder{
		
		public static function build(source:Source, raw:Array, forSync:Boolean=false, comboId:String=''):Array{
			if(!source || !raw || raw.length==0) return [];
			
			var result:Array=[];
			var order:Object;// :Order;
			var einfo:OrderExtraInfo;
			var src_id:int;
			var syncDate:Date= new Date();
			//var jMap:Array=AttrJsonMapDAO.getOrderJson(source.type);
			//var ejMap:Array=AttrJsonMapDAO.getOrderExtraJson(source.type);
			var jMap:Array= AttrJsonMap.getOrderJson(source.type);
			var ejMap:Array=AttrJsonMap.getOrderExtraJson(source.type);
			if(!jMap) return null;
			for each (var jo:Object in raw){
				if(jo){
					src_id=0;
					if(forSync){
						order=new OrderTemp();
						order.source=source.id;
						order.state=OrderState.FTP_WAITE;
						order.state_date=syncDate;
					}else{
						order=new Order();
					}
					//var src_id:int;
					var o:Object;
					var ajm:AttrJsonMap;
					var val:Object;
					var d:Date;
					
					//regular data
					for each(o in jMap){
						ajm=o as AttrJsonMap;
						if(ajm){
							if(order.hasOwnProperty(ajm.field)){
								//params array
								val=JsonUtil.getRawVal(ajm.json_key, jo);
								if(val){
									if(ajm.field.indexOf('date')!=-1){
										//convert date
										d=JsonUtil.parseDate(val.toString());
										order[ajm.field]=d;
									}else{
										order[ajm.field]=val;
									}
									if(ajm.field=='src_id'){
										//create id
										//removes subNumber (-#) for fotokniga
										if (val is String){
											src_id=cleanId(val as String);
										}else{
											src_id=int(val);
										}
										if(src_id) order.id=source.id.toString()+'_'+src_id.toString();
									}
									
								}
							}
						}
					}
					
					if(!forSync){
						einfo=new OrderExtraInfo();
						//extra info
						if(ejMap && ejMap.length>0){
							for each(o in ejMap){
								ajm=o as AttrJsonMap;
								if(ajm){
									if(einfo.hasOwnProperty(ajm.field)){
										//params array
										val=JsonUtil.getRawVal(ajm.json_key, jo);
										if(val){
											if(ajm.field.indexOf('date')!=-1){
												//convert date
												d=JsonUtil.parseDate(val.toString());
												einfo[ajm.field]=d;
											}else{
												einfo[ajm.field]=val;
											}
										}
									}
								}
							}
						}
						if(!einfo.isEmpty){
							einfo.cover=StrUtil.siteCode2Char(einfo.cover);
							//einfo.coverMaterial=StrUtil.siteCode2Char(einfo.coverMaterial);
							order.extraInfo=einfo;
						}
						
						//parse suborders
						var subOrder:SubOrder;
						if (source.type==SourceType.SRC_FBOOK && jo.hasOwnProperty('items') && jo.items is Array){
							var subMap:Array=AttrJsonMap.getSubOrderJson(source.type);
							var subRaw:Array= jo.items as Array;
							if(subRaw && subRaw.length>0 && subMap && subMap.length>0){
								for each(var so:Object in subRaw){
									subOrder = new SubOrder();
									for each(ajm in subMap){
										if(subOrder.hasOwnProperty(ajm.field)){
											//params array
											val=JsonUtil.getRawVal(ajm.json_key, so);
											if(val!=null){
												if(ajm.field.indexOf('date')!=-1){
													//convert date
													d=JsonUtil.parseDate(val.toString());
													subOrder[ajm.field]=d;
												}else{
													subOrder[ajm.field]=val;
												}
											}
										}
									}
									/*
									if(subOrder.native_type==1){
									//foto print, reset root ftp folder
									order.ftp_folder=subOrder.ftp_folder;
									}else{
									order.addSuborder(subOrder);
									}
									*/
									subOrder.projectIds.push(subOrder.sub_id);
									order.addSuborder(subOrder);
								}
							}
							if(!order.ftp_folder) order.ftp_folder=order.id;
						}else if(source.type==SourceType.SRC_FOTOKNIGA){
							order.src_id=comboId;
							//get subOrder id (-#)
							var subId:String; //:int;
							var a:Array=order.src_id.split('-');
							if(a && a.length>=2) subId=a[1];
							if(subId){
								subOrder= new SubOrder();
								subOrder.order_id=order.id;
								subOrder.sub_id=subId;
								subOrder.src_type=SourceType.SRC_FBOOK;
								if (jo.hasOwnProperty('projects') && jo.projects is Array && (jo.projects as Array).length>0){
									//multy book
									for each(subId in jo.projects) subOrder.projectIds.push(subId);
									subOrder.prt_qty=subOrder.projectIds.length;//book_mun
									order.addSuborder(subOrder);
								}else{
									//simple project
									if(order.fotos_num>0) subOrder.prt_qty=order.fotos_num;
									subOrder.projectIds.push(subOrder.sub_id);
									order.addSuborder(subOrder);
								}
							}
						}
						
					}
					if(!forSync || order.id) result.push(order); //skip if id empty
				}
			}
			return result;
		}
		
		private static function cleanId(src_id:String):int{
			//removes subNumber (-#) for fotokniga
			var a:Array=src_id.split('-');
			var sId:String;
			if(!a || a.length==0){
				sId=src_id;
			}else{
				sId=a[0];
			}
			return int(sId);
		}
		
		public static function saveToFilesystem(order:Order):int{
			if(!order) return OrderState.ERR_APP_INIT;
			if(!order.hasSuborders) return order.state;
			
			var src:Source=Context.getSource(order.source);
			if(!src) return OrderState.ERR_APP_INIT;
			
			//check wrk folder
			if(!Context.getAttribute('workFolder')) return OrderState.ERR_APP_INIT;
			//get order path
			var orderPath:String=src.getWrkFolder()+File.separator+order.ftp_folder;
			var orderFolder:File=new File(orderPath);
			if(!orderFolder.exists || !orderFolder.isDirectory){
				order.state=OrderState.ERR_FILE_SYSTEM;
				return order.state;
			}
			
			//save so projects
			var so:SubOrder;
			var soFolders:Object= new Object;
			var folder:File;
			var raws:Array;
			var file:File;
			if(order.hasSuborders){
				for each(so in order.suborders){
					if(so && so.ftp_folder && so.projects && so.projects.length>0){
						folder=orderFolder.resolvePath(so.ftp_folder);
						if(!folder.exists || !folder.isDirectory){
							order.state=OrderState.ERR_FILE_SYSTEM;
							return order.state;
						}
						//get raws
						raws=[];
						for each (var prj:FBookProject in so.projects){
							if(prj && prj.rawProject){
								raws.push(prj.rawProject);
							}
						}
						if(raws.length>0){
							//write array of raw projekts
							file=folder.resolvePath(so.sub_id+'.prj');
							try{
								if(file.exists && file.isDirectory){
									file.deleteDirectory(true);
								}
								var stream:FileStream;
								stream=new FileStream();
								stream.open(file,FileMode.WRITE);
								stream.writeObject(raws);
								stream.close();
							}catch(error:Error){
								order.state=OrderState.ERR_FILE_SYSTEM;
								return order.state;
							}
							
						}
					}
				}
			}
			return order.state;
		}
		
		public static function restoreFromFilesystem(order:Order):int{
			//TODO reset to load if file system error?
			if(!order) return OrderState.ERR_APP_INIT;
			var src:Source=Context.getSource(order.source);
			if(!src) return OrderState.ERR_APP_INIT;
			
			//check wrk folder
			if(!Context.getAttribute('workFolder')) return OrderState.ERR_APP_INIT;
			//get order path
			var orderPath:String=src.getWrkFolder()+File.separator+order.ftp_folder;
			var orderFolder:File=new File(orderPath);
			if(!orderFolder.exists || !orderFolder.isDirectory){
				order.state=OrderState.ERR_GET_PROJECT;
				return order.state;
			}
			
			//get first level listing
			var arr:Array=orderFolder.getDirectoryListing();
			if(!arr || arr.length==0){
				order.state=OrderState.ERR_GET_PROJECT;
				return order.state;
			}
			//build so folders map and reload so
			var so:SubOrder;
			var soFolders:Object= new Object;
			var folder:File;
			var listing:Array;
			var files:Array;
			var raws:Array;
			var file:File;
			if(order.hasSuborders){
				for each(so in order.suborders){
					if(so && so.ftp_folder){
						//load projects
						folder=orderFolder.resolvePath(so.ftp_folder);
						if(!folder.exists || !folder.isDirectory){
							order.state=OrderState.ERR_GET_PROJECT;
							return order.state;
						}
						//read array of raw projekts
						file=folder.resolvePath(so.sub_id+'.prj');
						if(!file.exists || file.isDirectory){
							order.state=OrderState.ERR_GET_PROJECT;
							return order.state;
						}
						var stream:FileStream;
						stream=new FileStream();
						try{
							stream.open(file,FileMode.READ);
							raws=stream.readObject() as Array;
							stream.close();
						}catch(error:Error){
							order.state=OrderState.ERR_GET_PROJECT;
							return order.state;
						}
						if(!raws || raws.length==0){
							order.state=OrderState.ERR_GET_PROJECT;
							return order.state;
						}
						
						so.projects=[];
						for each (var raw:Object in raws){
							so.projects.push(new FBookProject(raw));	
						}
						
						soFolders[so.ftp_folder]=true;
					}
				}
			}
			
			//fill order file structure
			var foldersMap:Dictionary= new Dictionary();
			for each(folder in arr){
				if(folder && folder.isDirectory){
					if(!soFolders[folder.name]){
						files=[];
						listing=folder.getDirectoryListing();
						if(listing && listing.length>0){
							for each(file in listing){
								if (file && !file.isDirectory) files.push(file.name);
							}
						}
						if(files.length>0){
							foldersMap[folder.name]=files;
						}
					}
				}
			}
			
			//create order printgruops
			//build order print groups 
			var pgBuilder:PrintGroupBuilder= new PrintGroupBuilder();
			var pgArr:Array;
			try{
				pgArr= pgBuilder.build(src,foldersMap,order.id);
			}catch (e:Error){
				trace('OrderBuilder error while build print groups '+order.id);
				order.state=OrderState.ERR_READ_LOCK;
				//StateLog.log(OrderState.ERR_READ_LOCK,order.id,'','Блокировка чтения при парсе групп печати.'); 
				return order.state;
			}
			if(pgArr) order.printGroups= new ArrayCollection(pgArr);
			
			/* created after build
			//build suborders prs
			try{
				pgBuilder.buildFromSuborders(order);
			}catch (e:Error){
				trace('OrderBuilder error while build print groups'+order.id+', error: '+e.message);
				order.state=OrderState.ERR_READ_LOCK;
				return order.state;
			}
			*/
			
			//restore photos original names
			try{
				CaptionSetter.restoreFileCaption(order,src.getWrkFolder());
			}catch(err:Error){
			}
			
			return order.state;
		}
	}
}
